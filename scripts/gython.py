#!/usr/bin/env python3
"""
gython.py - Python to Go AST Transpiler for OpenWebUI Backend
Translates OpenWebUI Python models, routers, and Socket.IO handlers into idiomatic Go code.
"""

import ast
import argparse
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple


TYPE_MAP = {
    "str": "string",
    "int": "int64",
    "float": "float64",
    "bool": "bool",
    "dict": "map[string]any",
    "list": "[]any",
    "Any": "any",
    "None": "nil",
    "Request": "*http.Request",
    "Response": "http.ResponseWriter",
}


def to_pascal_case(s: str) -> str:
    parts = s.split("_")
    return "".join(p.capitalize() for p in parts if p)


def to_camel_case(s: str) -> str:
    pascal = to_pascal_case(s)
    if not pascal:
        return ""
    return pascal[0].lower() + pascal[1:]


class GoCodeGenerator(ast.NodeVisitor):
    def __init__(self, indent_level: int = 0):
        self.indent_level = indent_level
        self.lines: List[str] = []
        self.routes: List[Dict[str, str]] = []

    def indent(self) -> str:
        return "\t" * self.indent_level

    def add_line(self, line: str):
        self.lines.append(f"{self.indent()}{line}")

    def transpile_type(self, node: Optional[ast.AST]) -> str:
        if node is None:
            return "any"
        if isinstance(node, ast.Name):
            return TYPE_MAP.get(node.id, node.id)
        if isinstance(node, ast.Subscript):
            val_type = self.transpile_type(node.value)
            if isinstance(node.value, ast.Name):
                if node.value.id in ("dict", "Dict"):
                    return "map[string]any"
                if node.value.id in ("list", "List"):
                    elem = self.transpile_type(node.slice)
                    return f"[]{elem}"
                if node.value.id in ("Optional", "Union"):
                    elem = self.transpile_type(node.slice)
                    return f"*{elem}"
                if node.value.id in ("set", "Set"):
                    elem = self.transpile_type(node.slice)
                    return f"map[{elem}]bool"
            return f"[]{self.transpile_type(node.slice)}"
        if isinstance(node, ast.BinOp) and isinstance(node.op, ast.BitOr):
            # Python 3.10+ union: str | None -> *str
            left = self.transpile_type(node.left)
            right = self.transpile_type(node.right)
            if right in ("nil", "None", "any"):
                return f"*{left}"
            return left
        if isinstance(node, ast.Constant):
            if node.value is None:
                return "any"
        return "any"

    def transpile_expr(self, node: ast.AST) -> str:
        if isinstance(node, ast.Constant):
            if isinstance(node.value, str):
                escaped = node.value.replace('"', '\\"').replace("\n", "\\n")
                return f'"{escaped}"'
            elif isinstance(node.value, bool):
                return "true" if node.value else "false"
            elif node.value is None:
                return "nil"
            return str(node.value)

        elif isinstance(node, ast.Name):
            if node.id == "True":
                return "true"
            if node.id == "False":
                return "false"
            if node.id == "None":
                return "nil"
            if node.id == "self":
                return "h"
            return node.id

        elif isinstance(node, ast.Attribute):
            val = self.transpile_expr(node.value)
            # Python string methods
            if node.attr == "startswith":
                return "strings.HasPrefix"
            if node.attr == "endswith":
                return "strings.HasSuffix"
            if node.attr == "split":
                return "strings.Split"
            return f"{val}.{node.attr}"

        elif isinstance(node, ast.Subscript):
            val = self.transpile_expr(node.value)
            sl = self.transpile_expr(node.slice)
            return f"{val}[{sl}]"

        elif isinstance(node, ast.UnaryOp):
            op = "!" if isinstance(node.op, ast.Not) else "-"
            operand = self.transpile_expr(node.operand)
            return f"{op}({operand})"

        elif isinstance(node, ast.BinOp):
            left = self.transpile_expr(node.left)
            right = self.transpile_expr(node.right)
            op = "+"
            if isinstance(node.op, ast.Sub):
                op = "-"
            elif isinstance(node.op, ast.Mult):
                op = "*"
            elif isinstance(node.op, ast.Div):
                op = "/"
            return f"{left} {op} {right}"

        elif isinstance(node, ast.Compare):
            left = self.transpile_expr(node.left)
            parts = [left]
            for op, comparator in zip(node.ops, node.comparators):
                comp_str = self.transpile_expr(comparator)
                if isinstance(op, ast.Eq):
                    parts.append(f"== {comp_str}")
                elif isinstance(op, ast.NotEq):
                    parts.append(f"!= {comp_str}")
                elif isinstance(op, ast.Lt):
                    parts.append(f"< {comp_str}")
                elif isinstance(op, ast.LtE):
                    parts.append(f"<= {comp_str}")
                elif isinstance(op, ast.Gt):
                    parts.append(f"> {comp_str}")
                elif isinstance(op, ast.GtE):
                    parts.append(f">= {comp_str}")
                elif isinstance(op, ast.In):
                    return f"slices.Contains({comp_str}, {left})"
                elif isinstance(op, ast.NotIn):
                    return f"!slices.Contains({comp_str}, {left})"
            return " ".join(parts)

        elif isinstance(node, ast.BoolOp):
            op = " && " if isinstance(node.op, ast.And) else " || "
            return "(" + op.join(self.transpile_expr(v) for v in node.values) + ")"

        elif isinstance(node, ast.JoinedStr):
            # f-string: f"user:{user.id}" -> fmt.Sprintf("user:%v", user.id)
            fmt_parts = []
            args = []
            for part in node.values:
                if isinstance(part, ast.Constant):
                    fmt_parts.append(part.value.replace("%", "%%"))
                elif isinstance(part, ast.FormattedValue):
                    fmt_parts.append("%v")
                    args.append(self.transpile_expr(part.value))
            fmt_str = "".join(fmt_parts)
            if not args:
                return f'"{fmt_str}"'
            return f'fmt.Sprintf("{fmt_str}", {", ".join(args)})'

        elif isinstance(node, ast.Dict):
            items = []
            for k, v in zip(node.keys, node.values):
                if k is None:
                    continue
                k_str = self.transpile_expr(k)
                v_str = self.transpile_expr(v)
                items.append(f"{k_str}: {v_str}")
            return "map[string]any{" + ", ".join(items) + "}"

        elif isinstance(node, ast.List):
            elems = [self.transpile_expr(e) for e in node.elts]
            return "[]any{" + ", ".join(elems) + "}"

        elif isinstance(node, ast.Set):
            elems = [self.transpile_expr(e) for e in node.elts]
            return "map[string]bool{" + ", ".join(f"{e}: true" for e in elems) + "}"

        elif isinstance(node, ast.ListComp):
            # [expr for x in iter if cond]
            elt = self.transpile_expr(node.elt)
            gen = node.generators[0]
            target = self.transpile_expr(gen.target)
            iter_expr = self.transpile_expr(gen.iter)
            cond_str = ""
            if gen.ifs:
                cond_str = f"if {self.transpile_expr(gen.ifs[0])} "
            return f"/* ListComp: for _, {target} := range {iter_expr} {cond_str}{{ res = append(res, {elt}) }} */"

        elif isinstance(node, ast.DictComp):
            k = self.transpile_expr(node.key)
            v = self.transpile_expr(node.value)
            gen = node.generators[0]
            target = self.transpile_expr(gen.target)
            iter_expr = self.transpile_expr(gen.iter)
            return f"/* DictComp: for _, {target} := range {iter_expr} {{ m[{k}] = {v} }} */"

        elif isinstance(node, ast.SetComp):
            elt = self.transpile_expr(node.elt)
            gen = node.generators[0]
            target = self.transpile_expr(gen.target)
            iter_expr = self.transpile_expr(gen.iter)
            return f"/* SetComp: for _, {target} := range {iter_expr} {{ s[{elt}] = true }} */"

        elif isinstance(node, ast.Await):
            return self.transpile_expr(node.value)

        elif isinstance(node, ast.Call):
            func_name = ""
            if isinstance(node.func, ast.Name):
                func_name = node.func.id
            elif isinstance(node.func, ast.Attribute):
                val_name = self.transpile_expr(node.func.value)
                attr = node.func.attr
                func_name = f"{val_name}.{attr}"
            else:
                try:
                    func_name = ast.unparse(node.func)
                except Exception:
                    func_name = "func"

            args = [self.transpile_expr(a) for a in node.args]

            # Model & DB transforms
            if func_name == "Chats.update_chat_last_read_at_by_id":
                return f"h.db.UpdateOpenWebUIChatLastReadAt(subdomain, userID, {args[0]})"
            if func_name == "Folders.get_folders_by_user_id":
                return f"h.db.GetOpenWebUIFolders(subdomain, {args[0]})"
            if func_name == "Chats.count_unread_by_folder_ids":
                return f"h.db.CountOpenWebUIUnreadByFolder(subdomain, {args[0]})"
            if func_name == "Users.update_last_active_by_id":
                return f"h.db.UpdateUserLastActive(subdomain, {args[0]})"
            if func_name in ("time.time", "time.time_ns"):
                return "time.Now().Unix()"
            if func_name == "uuid.uuid4":
                return "uuid.New().String()"
            if func_name == "dict.fromkeys":
                return f"make(map[string]int)"

            # Socket IO
            if "sio.emit" in func_name:
                evt = args[0] if args else '""'
                data = args[1] if len(args) > 1 else "nil"
                room = "room"
                for kw in node.keywords:
                    if kw.arg in ("room", "to"):
                        room = self.transpile_expr(kw.value)
                return f"globalSocketHub.broadcastToRoom({room}, {evt}, {data})"

            if "sio.enter_room" in func_name:
                return f"globalSocketHub.joinRoom(client, {args[1]})"

            if "sio.leave_room" in func_name:
                return f"globalSocketHub.leaveRoom(client, {args[1]})"

            # Dict .get(key, default)
            if func_name.endswith(".get") and len(args) >= 1:
                obj = func_name[:-4]
                default_val = args[1] if len(args) > 1 else "nil"
                return f"/* {obj}.get({args[0]}, {default_val}) */"

            return f"{func_name}({', '.join(args)})"

        return f"/* {ast.dump(node)} */"

    def visit_ClassDef(self, node: ast.ClassDef):
        """Transpiles Pydantic BaseModel classes to Go structs."""
        base_names = [ast.unparse(b) for b in node.bases]
        is_model = any(b in ("BaseModel", "ConfigDict") or "Model" in b or "Response" in b or "Form" in b for b in base_names)
        
        struct_name = node.name
        self.add_line(f"// {struct_name} represents {node.name}")
        self.add_line(f"type {struct_name} struct {{")
        self.indent_level += 1

        for item in node.body:
            if isinstance(item, ast.AnnAssign) and isinstance(item.target, ast.Name):
                field_name = item.target.id
                go_field = to_pascal_case(field_name)
                go_type = self.transpile_type(item.annotation)
                tag = f'`json:"{field_name}"`'
                if "Optional" in ast.unparse(item.annotation) or (item.value and isinstance(item.value, ast.Constant) and item.value.value is None):
                    tag = f'`json:"{field_name},omitempty"`'
                self.add_line(f"{go_field} {go_type} {tag}")
            elif isinstance(item, ast.Assign):
                for target in item.targets:
                    if isinstance(target, ast.Name):
                        field_name = target.id
                        if field_name in ("model_config", "__tablename__"):
                            continue
                        go_field = to_pascal_case(field_name)
                        tag = f'`json:"{field_name},omitempty"`'

                        # Check for SQLAlchemy Column(...)
                        if isinstance(item.value, ast.Call) and (getattr(item.value.func, "id", "") == "Column" or getattr(item.value.func, "attr", "") == "Column"):
                            sql_type = "any"
                            if item.value.args:
                                arg0 = ast.unparse(item.value.args[0])
                                if "Text" in arg0 or "String" in arg0:
                                    sql_type = "string"
                                elif "BigInteger" in arg0 or "Integer" in arg0:
                                    sql_type = "int64"
                                elif "Boolean" in arg0:
                                    sql_type = "bool"
                                elif "JSON" in arg0:
                                    sql_type = "map[string]any"
                            is_nullable = False
                            for kw in item.value.keywords:
                                if kw.arg == "nullable" and isinstance(kw.value, ast.Constant) and kw.value.value is True:
                                    is_nullable = True
                            if is_nullable and sql_type != "map[string]any" and not sql_type.startswith("*"):
                                sql_type = f"*{sql_type}"
                            self.add_line(f"{go_field} {sql_type} {tag}")
                        else:
                            self.add_line(f"{go_field} any {tag}")

        self.indent_level -= 1
        self.add_line("}")
        self.add_line("")

    def visit_FunctionDef(self, node):
        self._transpile_function(node)

    def visit_AsyncFunctionDef(self, node):
        self._transpile_function(node)

    def _transpile_function(self, node):
        func_name = node.name
        go_name = to_pascal_case(func_name)

        # Check for FastAPI route decorators
        route_method = None
        route_path = None
        for dec in node.decorator_list:
            if isinstance(dec, ast.Call) and isinstance(dec.func, ast.Attribute):
                if isinstance(dec.func.value, ast.Name) and dec.func.value.id == "router":
                    route_method = dec.func.attr.upper()
                    if dec.args:
                        route_path = self.transpile_expr(dec.args[0]).strip('"')
                    self.routes.append({
                        "method": route_method,
                        "path": route_path or "/",
                        "handler": go_name,
                        "py_func": func_name
                    })

        params = []
        for arg in node.args.args:
            p_name = arg.arg
            if p_name in ("self", "cls"):
                continue
            p_type = self.transpile_type(arg.annotation)
            if p_name == "sid":
                p_name = "client"
                p_type = "*SocketClient"
            elif p_name == "data":
                p_type = "map[string]any"
            elif p_name == "request":
                p_name = "r"
                p_type = "*http.Request"
            elif p_name in ("user", "auth_user"):
                p_name = "user"
                p_type = "*OpenWebUIUser"
            params.append(f"{p_name} {p_type}")

        ret_type = ""
        if node.returns:
            ret_type = " " + self.transpile_type(node.returns)
        elif any(isinstance(sub, ast.Return) and sub.value is not None for sub in ast.walk(node)):
            ret_type = " any"

        # Docstring
        doc = ast.get_docstring(node)
        if doc:
            for d_line in doc.strip().split("\n"):
                self.add_line(f"// {d_line}")

        if route_method and route_path:
            self.add_line(f"// Route: {route_method} {route_path}")

        self.add_line(f"func (h *APIHandler) {go_name}({', '.join(params)}){ret_type} {{")
        self.indent_level += 1

        for stmt in node.body:
            if isinstance(stmt, ast.Expr) and isinstance(stmt.value, ast.Constant) and isinstance(stmt.value.value, str):
                continue
            self.visit(stmt)

        self.indent_level -= 1
        self.add_line("}")
        self.add_line("")

    def visit_If(self, node: ast.If):
        cond = self.transpile_expr(node.test)
        self.add_line(f"if {cond} {{")
        self.indent_level += 1
        for s in node.body:
            self.visit(s)
        self.indent_level -= 1

        if node.orelse:
            if len(node.orelse) == 1 and isinstance(node.orelse[0], ast.If):
                self.add_line("} else ")
                self.visit(node.orelse[0])
            else:
                self.add_line("} else {")
                self.indent_level += 1
                for s in node.orelse:
                    self.visit(s)
                self.indent_level -= 1
                self.add_line("}")
        else:
            self.add_line("}")

    def visit_Assign(self, node: ast.Assign):
        targets = [self.transpile_expr(t) for t in node.targets]
        val = self.transpile_expr(node.value)
        self.add_line(f"{', '.join(targets)} := {val}")

    def visit_AugAssign(self, node: ast.AugAssign):
        target = self.transpile_expr(node.target)
        val = self.transpile_expr(node.value)
        op = "+="
        if isinstance(node.op, ast.Sub):
            op = "-="
        self.add_line(f"{target} {op} {val}")

    def visit_For(self, node: ast.For):
        target = self.transpile_expr(node.target)
        iter_expr = self.transpile_expr(node.iter)
        self.add_line(f"for _, {target} := range {iter_expr} {{")
        self.indent_level += 1
        for s in node.body:
            self.visit(s)
        self.indent_level -= 1
        self.add_line("}")

    def visit_While(self, node: ast.While):
        cond = self.transpile_expr(node.test)
        self.add_line(f"for {cond} {{")
        self.indent_level += 1
        for s in node.body:
            self.visit(s)
        self.indent_level -= 1
        self.add_line("}")

    def visit_Return(self, node: ast.Return):
        if node.value is None:
            self.add_line("return")
        else:
            val = self.transpile_expr(node.value)
            self.add_line(f"return {val}")

    def visit_Expr(self, node: ast.Expr):
        expr_str = self.transpile_expr(node.value)
        if expr_str:
            self.add_line(f"{expr_str}")


def transpile_file(file_path: Path, target_func: Optional[str] = None, target_class: Optional[str] = None) -> Tuple[str, List[Dict[str, str]]]:
    with open(file_path, "r", encoding="utf-8") as f:
        code = f.read()

    tree = ast.parse(code, filename=str(file_path))
    gen = GoCodeGenerator()

    for node in tree.body:
        if isinstance(node, ast.ClassDef):
            if target_func:
                continue
            if target_class and node.name != target_class:
                continue
            gen.visit(node)
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if target_class:
                continue
            if target_func and node.name != target_func:
                continue
            gen.visit(node)

    return "\n".join(gen.lines), gen.routes


def run_inventory(routers_dir: Path, models_dir: Path):
    print("=" * 80)
    print("OpenWebUI Full Backend Inventory (Routers & Models)")
    print("=" * 80)
    total_routes = 0
    total_models = 0

    router_files = sorted(routers_dir.glob("*.py"))
    print(f"\n📁 Routers ({len(router_files)} files):")
    for rf in router_files:
        if rf.name == "__init__.py":
            continue
        try:
            _, routes = transpile_file(rf)
            total_routes += len(routes)
            methods = set(r["method"] for r in routes)
            print(f"  • {rf.name:<20} {len(routes):>2} route(s)  [{', '.join(sorted(methods))}]")
        except Exception as e:
            print(f"  • {rf.name:<20} ERROR: {e}")

    model_files = sorted(models_dir.glob("*.py"))
    print(f"\n📁 Models ({len(model_files)} files):")
    for mf in model_files:
        if mf.name == "__init__.py":
            continue
        try:
            with open(mf, "r", encoding="utf-8") as f:
                tree = ast.parse(f.read())
            classes = [n.name for n in tree.body if isinstance(n, ast.ClassDef)]
            total_models += len(classes)
            print(f"  • {mf.name:<20} {len(classes):>2} class(es): {', '.join(classes[:4])}{'...' if len(classes) > 4 else ''}")
        except Exception as e:
            print(f"  • {mf.name:<20} ERROR: {e}")

    print("\n" + "=" * 80)
    print(f"TOTALS: {total_routes} REST endpoints across {len(router_files)} routers, {total_models} models across {len(model_files)} model files")
    print("=" * 80)


def port_all_routers(routers_dir: Path, out_dir: Path):
    out_dir.mkdir(parents=True, exist_ok=True)
    router_files = sorted(routers_dir.glob("*.py"))
    print(f"Porting {len(router_files)} routers to {out_dir}...")
    for rf in router_files:
        if rf.name == "__init__.py":
            continue
        try:
            code, routes = transpile_file(rf)
            if not code.strip():
                continue
            base_name = rf.stem
            out_file = out_dir / f"openwebui_{base_name}_gen.go"
            header = f"// Code generated by gython.py from {rf.name}. DO NOT EDIT MANUALLY.\npackage api\n\nimport (\n\t\"net/http\"\n\t\"slices\"\n\t\"strings\"\n\t\"time\"\n)\n\n"
            with open(out_file, "w", encoding="utf-8") as f:
                f.write(header + code + "\n")
            print(f"  ✓ {rf.name} -> {out_file.name} ({len(routes)} routes)")
        except Exception as e:
            print(f"  ✗ {rf.name}: {e}")


def main():
    parser = argparse.ArgumentParser(description="gython.py - Python to Go AST Transpiler for OpenWebUI")
    parser.add_argument("--file", default="/tmp/open-webui/backend/open_webui/socket/main.py", help="Python source file to transpile")
    parser.add_argument("--func", help="Specific function name to transpile")
    parser.add_argument("--class", dest="class_name", help="Specific class name to transpile")
    parser.add_argument("--all", action="store_true", help="Transpile all functions and classes in file")
    parser.add_argument("--routes", action="store_true", help="List detected FastAPI routes")
    parser.add_argument("--inventory", action="store_true", help="Scan full OpenWebUI backend inventory")
    parser.add_argument("--port-dir", help="Directory to output transpiled Go files")
    parser.add_argument("--output", help="Optional output Go file")

    args = parser.parse_args()

    routers_dir = Path("/tmp/open-webui/backend/open_webui/routers")
    models_dir = Path("/tmp/open-webui/backend/open_webui/models")

    if args.inventory:
        run_inventory(routers_dir, models_dir)
        return

    if args.port_dir:
        port_all_routers(routers_dir, Path(args.port_dir))
        return

    py_file = Path(args.file)
    if not py_file.exists():
        print(f"Error: file {py_file} not found.", file=sys.stderr)
        sys.exit(1)

    result_code, routes = transpile_file(py_file, target_func=args.func, target_class=args.class_name)

    if args.routes:
        print(f"Detected {len(routes)} route(s) in {py_file}:")
        for r in routes:
            print(f"  [{r['method']}] {r['path']} -> {r['handler']} ({r['py_func']})")
        return

    if args.output:
        with open(args.output, "w", encoding="utf-8") as f:
            f.write(result_code)
        print(f"Wrote transpiled Go code to {args.output}")
    else:
        print(result_code)


if __name__ == "__main__":
    main()
