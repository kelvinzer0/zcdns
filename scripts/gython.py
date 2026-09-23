#!/usr/bin/env python3
"""
gython.py - Python to Go AST Transpiler for OpenWebUI Backend
Translates OpenWebUI Python functions and Socket.IO handlers into idiomatic Go code.
"""

import ast
import argparse
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional


TYPE_MAP = {
    "str": "string",
    "int": "int64",
    "float": "float64",
    "bool": "bool",
    "dict": "map[string]any",
    "list": "[]any",
    "Any": "any",
    "None": "nil",
}


class GoCodeGenerator(ast.NodeVisitor):
    def __init__(self, indent_level: int = 0):
        self.indent_level = indent_level
        self.lines: List[str] = []

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
            val = self.transpile_type(node.value)
            if val == "dict" or isinstance(node.value, ast.Name) and node.value.id == "dict":
                return "map[string]any"
            if val == "list" or isinstance(node.value, ast.Name) and node.value.id == "list":
                elem = self.transpile_type(node.slice)
                return f"[]{elem}"
            if isinstance(node.value, ast.Name) and node.value.id == "Optional":
                elem = self.transpile_type(node.slice)
                return f"*{elem}"
        if isinstance(node, ast.Constant):
            if node.value is None:
                return "any"
        return "any"

    def transpile_expr(self, node: ast.AST) -> str:
        if isinstance(node, ast.Constant):
            if isinstance(node.value, str):
                escaped = node.value.replace('"', '\\"')
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
                return f"strings.HasPrefix"
            if node.attr == "split":
                return f"strings.Split"
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

            # Specific OpenWebUI DB / Model transformations
            if func_name == "Chats.update_chat_last_read_at_by_id":
                return f"h.db.UpdateOpenWebUIChatLastReadAt(subdomain, userID, {args[0]})"
            if func_name == "Folders.get_folders_by_user_id":
                return f"h.db.GetOpenWebUIFolders(subdomain, {args[0]})"
            if func_name == "Chats.count_unread_by_folder_ids":
                return f"h.db.CountOpenWebUIUnreadByFolder(subdomain, {args[0]})"
            if func_name == "Users.update_last_active_by_id":
                return f"h.db.UpdateUserLastActive(subdomain, {args[0]})"
            if func_name == "time.time":
                return "time.Now().Unix()"

            # sio.emit
            if "sio.emit" in func_name:
                evt = args[0] if args else '""'
                data = args[1] if len(args) > 1 else "nil"
                room = "room"
                for kw in node.keywords:
                    if kw.arg in ("room", "to"):
                        room = self.transpile_expr(kw.value)
                return f"globalSocketHub.broadcastToRoom({room}, {evt}, {data})"

            # sio.enter_room
            if "sio.enter_room" in func_name:
                return f"globalSocketHub.joinRoom(client, {args[1]})"

            # sio.leave_room
            if "sio.leave_room" in func_name:
                return f"globalSocketHub.leaveRoom(client, {args[1]})"

            return f"{func_name}({', '.join(args)})"

        return f"/* {ast.dump(node)} */"

    def visit_FunctionDef(self, node):
        self._transpile_function(node)

    def visit_AsyncFunctionDef(self, node):
        self._transpile_function(node)

    def _transpile_function(self, node):
        func_name = node.name
        # Convert snake_case to PascalCase
        go_name = "".join(part.capitalize() for part in func_name.split("_"))
        
        # Determine parameters
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

        self.add_line(f"func (h *APIHandler) {go_name}({', '.join(params)}){ret_type} {{")
        self.indent_level += 1

        for stmt in node.body:
            if isinstance(stmt, ast.Expr) and isinstance(stmt.value, ast.Constant) and isinstance(stmt.value.value, str):
                continue  # skip raw docstring
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
                # will print `if ... {`
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


def main():
    parser = argparse.ArgumentParser(description="gython.py - Python to Go Transpiler for OpenWebUI")
    parser.add_argument("--file", default="/tmp/open-webui/backend/open_webui/socket/main.py", help="Python source file to transpile")
    parser.add_argument("--func", help="Specific function name to transpile")
    parser.add_argument("--all", action="store_true", help="Transpile all functions in file")
    parser.add_argument("--output", help="Optional output Go file")

    args = parser.parse_args()

    py_file = Path(args.file)
    if not py_file.exists():
        print(f"Error: file {py_file} not found.", file=sys.stderr)
        sys.exit(1)

    with open(py_file, "r", encoding="utf-8") as f:
        code = f.read()

    tree = ast.parse(code, filename=str(py_file))

    gen = GoCodeGenerator()

    matched = 0
    for node in tree.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            if args.func and node.name != args.func:
                continue
            gen.visit(node)
            matched += 1

    result_code = "\n".join(gen.lines)

    if args.output:
        with open(args.output, "w", encoding="utf-8") as f:
            f.write(result_code)
        print(f"Wrote {matched} transpiled function(s) to {args.output}")
    else:
        print(result_code)


if __name__ == "__main__":
    main()
