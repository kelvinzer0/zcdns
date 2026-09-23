#!/usr/bin/env python3
"""
Deep AST Logic Scanner for OpenWebUI Socket.IO
Analyzes control flow, state mutations, database operations, room events,
and event emissions in backend/open_webui/socket/main.py.
Outputs a structured logic specification for Golang conversion.
"""

import ast
import json
import os
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional

SOCKET_MAIN_PATH = Path("/tmp/open-webui/backend/open_webui/socket/main.py")
OUTPUT_LOGIC_JSON = Path("/tmp/openwebui_socket_logic_spec.json")
OUTPUT_LOGIC_MD = Path("/tmp/openwebui_socket_logic_spec.md")


class LogicFlowAnalyzer(ast.NodeVisitor):
    def __init__(self, func_node: ast.AST):
        self.func_node = func_node
        self.db_operations: List[str] = []
        self.state_mutations: List[str] = []
        self.room_operations: List[str] = []
        self.sio_emits: List[Dict[str, Any]] = []
        self.sio_calls: List[Dict[str, Any]] = []
        self.branches: List[str] = []
        self.returned_data: Optional[str] = None

    def analyze(self) -> Dict[str, Any]:
        self.visit(self.func_node)
        return {
            "db_operations": sorted(list(set(self.db_operations))),
            "state_mutations": sorted(list(set(self.state_mutations))),
            "room_operations": sorted(list(set(self.room_operations))),
            "sio_emits": self.sio_emits,
            "sio_calls": self.sio_calls,
            "branches": self.branches,
            "return_value": self.returned_data,
        }

    def visit_If(self, node: ast.If):
        cond_str = ast.unparse(node.test)
        self.branches.append(f"if {cond_str}")
        self.generic_visit(node)

    def visit_Assign(self, node: ast.Assign):
        for target in node.targets:
            target_str = ast.unparse(target)
            if any(pool in target_str for pool in ("SESSION_POOL", "USAGE_POOL", "EVENT_QUEUES", "YDOC_MANAGER")):
                val_str = ast.unparse(node.value)[:80]
                self.state_mutations.append(f"{target_str} = {val_str}")
        self.generic_visit(node)

    def visit_Call(self, node: ast.Call):
        func_str = ""
        try:
            func_str = ast.unparse(node.func)
        except Exception:
            pass

        # Database operations
        for model in ("Chats.", "Folders.", "Channels.", "Users.", "Notes.", "AccessGrants.", "YDOC_MANAGER."):
            if model in func_str:
                args_str = ", ".join(ast.unparse(a)[:40] for a in node.args)
                self.db_operations.append(f"{func_str}({args_str})")

        # Room operations
        if "sio.enter_room" in func_str:
            args = [ast.unparse(a) for a in node.args]
            self.room_operations.append(f"enter_room({', '.join(args)})")
        elif "sio.leave_room" in func_str:
            args = [ast.unparse(a) for a in node.args]
            self.room_operations.append(f"leave_room({', '.join(args)})")

        # SIO Emit
        if "sio.emit" in func_str:
            evt_name = ast.unparse(node.args[0]) if node.args else "unknown"
            payload_str = ast.unparse(node.args[1]) if len(node.args) > 1 else "{}"
            room_str = None
            skip_sid = None
            for kw in node.keywords:
                if kw.arg in ("room", "to"):
                    room_str = ast.unparse(kw.value)
                elif kw.arg == "skip_sid":
                    skip_sid = ast.unparse(kw.value)

            self.sio_emits.append({
                "event": evt_name,
                "room": room_str,
                "skip_sid": skip_sid,
                "payload_code": payload_str[:200],
            })

        # SIO Call (RPC)
        if "sio.call" in func_str:
            evt_name = ast.unparse(node.args[0]) if node.args else "unknown"
            payload_str = ast.unparse(node.args[1]) if len(node.args) > 1 else "{}"
            to_str = None
            for kw in node.keywords:
                if kw.arg in ("to", "room"):
                    to_str = ast.unparse(kw.value)

            self.sio_calls.append({
                "event": evt_name,
                "to": to_str,
                "payload_code": payload_str[:200],
            })

        self.generic_visit(node)

    def visit_Return(self, node: ast.Return):
        if node.value is not None:
            try:
                self.returned_data = ast.unparse(node.value)
            except Exception:
                self.returned_data = "<value>"
        self.generic_visit(node)


def scan_socket_logic(file_path: Path) -> List[Dict[str, Any]]:
    with open(file_path, "r", encoding="utf-8") as f:
        code = f.read()

    tree = ast.parse(code, filename=str(file_path))
    results = []

    for node in tree.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            event_name = None
            is_handler = False
            for d in node.decorator_list:
                d_str = ast.unparse(d)
                if "sio.on" in d_str:
                    is_handler = True
                    if isinstance(d, ast.Call) and d.args:
                        event_name = ast.unparse(d.args[0]).strip("'\"")
                elif "sio.event" in d_str:
                    is_handler = True
                    event_name = node.name

            analyzer = LogicFlowAnalyzer(node)
            logic_data = analyzer.analyze()

            doc = ast.get_docstring(node) or ""

            results.append({
                "name": node.name,
                "is_socket_handler": is_handler,
                "socket_event": event_name,
                "doc": doc.strip(),
                "line": node.lineno,
                "logic": logic_data,
            })

    return results


def main():
    print("=" * 65)
    print("  OPENWEBUI DEEP SOCKET.IO LOGIC SCANNER")
    print("=" * 65)

    if not SOCKET_MAIN_PATH.exists():
        print(f"Error: {SOCKET_MAIN_PATH} does not exist!")
        sys.exit(1)

    print(f"Parsing AST from: {SOCKET_MAIN_PATH}")
    functions = scan_socket_logic(SOCKET_MAIN_PATH)

    handlers = [f for f in functions if f["is_socket_handler"]]
    helpers = [f for f in functions if not f["is_socket_handler"]]

    print(f"Found {len(handlers)} Socket.IO event handlers and {len(helpers)} core logic helper functions.\n")

    # Save to JSON
    output_spec = {
        "socket_handlers": handlers,
        "helper_logic": helpers,
    }
    with open(OUTPUT_LOGIC_JSON, "w", encoding="utf-8") as f:
        json.dump(output_spec, f, indent=2)

    # Generate Markdown documentation
    lines = [
        "# OpenWebUI Socket.IO Deep Logic Specification",
        "Generated via Python AST static analysis from `backend/open_webui/socket/main.py`.\n",
        "## 1. Socket.IO Event Handlers Logic\n",
    ]

    for h in handlers:
        lines.append(f"### Event: `{h['socket_event']}` (Function `{h['name']}` at L{h['line']})")
        if h["doc"]:
            lines.append(f"**Doc**: {h['doc']}\n")
        
        logic = h["logic"]
        lines.append(f"- **Return / ACK**: `{logic['return_value'] or 'None'}`")
        
        if logic["branches"]:
            lines.append("- **Condition Branches**:")
            for b in logic["branches"]:
                lines.append(f"  - `{b}`")
                
        if logic["db_operations"]:
            lines.append("- **Database Operations**:")
            for db in logic["db_operations"]:
                lines.append(f"  - `{db}`")
                
        if logic["state_mutations"]:
            lines.append("- **State Mutations**:")
            for st in logic["state_mutations"]:
                lines.append(f"  - `{st}`")

        if logic["room_operations"]:
            lines.append("- **Room Operations**:")
            for ro in logic["room_operations"]:
                lines.append(f"  - `{ro}`")

        if logic["sio_emits"]:
            lines.append("- **Emits to Clients**:")
            for em in logic["sio_emits"]:
                lines.append(f"  - Event `{em['event']}` to room `{em['room']}` (skip: `{em['skip_sid']}`)")
                lines.append(f"    Payload: `{em['payload_code']}`")

        lines.append("")

    lines.append("## 2. Core Helper Logic Functions\n")
    for hp in helpers:
        if any(hp["logic"].values()):
            lines.append(f"### Helper: `{hp['name']}` (L{hp['line']})")
            if hp["doc"]:
                lines.append(f"**Doc**: {hp['doc']}")
            if hp["logic"]["db_operations"]:
                lines.append(f"- **DB Calls**: {', '.join(hp['logic']['db_operations'])}")
            if hp["logic"]["sio_emits"]:
                for em in hp["logic"]["sio_emits"]:
                    lines.append(f"- **Emits**: `{em['event']}` -> `{em['room']}`")
            lines.append("")

    with open(OUTPUT_LOGIC_MD, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))

    print(f"Successfully generated logic specs:")
    print(f"  -> JSON: {OUTPUT_LOGIC_JSON}")
    print(f"  -> Markdown: {OUTPUT_LOGIC_MD}")

    print("\n--- KEY LOGIC FLOW IDENTIFIED ---")
    for h in handlers:
        db_count = len(h["logic"]["db_operations"])
        emit_count = len(h["logic"]["sio_emits"])
        room_count = len(h["logic"]["room_operations"])
        print(f"  - {h['socket_event']:<25} | DB: {db_count} | Emits: {emit_count} | Rooms: {room_count}")

    print("\nDone.")


if __name__ == "__main__":
    main()
