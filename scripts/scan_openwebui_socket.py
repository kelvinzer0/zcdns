#!/usr/bin/env python3
"""
OpenWebUI Socket.IO Protocol Scanner
Scans Python backend and Svelte frontend to discover all Socket.IO messages,
requests, responses, ACKs, rooms, and stream channels.
Compares with Go implementation in backend/api/openwebui_socket.go.
"""

import ast
import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Dict, List, Set

BACKEND_DIR = Path("/tmp/open-webui/backend")
FRONTEND_DIR = Path("/tmp/open-webui/src")
GO_SOCKET_FILE = Path("/home/kelvinandriancom/www/backend/api/openwebui_socket.go")
OUTPUT_SPEC_FILE = Path("/tmp/openwebui_socket_spec.json")


class DataKeyExtractor(ast.NodeVisitor):
    def __init__(self, target_var: str = "data"):
        self.target_var = target_var
        self.keys = set()
        self.nested = {}

    def visit_Subscript(self, node):
        # data['key']
        if isinstance(node.value, ast.Name) and node.value.id == self.target_var:
            if isinstance(node.slice, ast.Constant) and isinstance(node.slice.value, str):
                self.keys.add(node.slice.value)
        self.generic_visit(node)

    def visit_Call(self, node):
        # data.get('key', default)
        if isinstance(node.func, ast.Attribute) and node.func.attr == "get":
            if isinstance(node.func.value, ast.Name) and node.func.value.id == self.target_var:
                if node.args and isinstance(node.args[0], ast.Constant) and isinstance(node.args[0].value, str):
                    self.keys.add(node.args[0].value)
        self.generic_visit(node)


class SioBackendVisitor(ast.NodeVisitor):
    def __init__(self):
        self.server_handlers: Dict[str, Dict[str, Any]] = {}
        self.server_emits: List[Dict[str, Any]] = []

    def visit_FunctionDef(self, node):
        self._check_handler(node)
        self.generic_visit(node)

    def visit_AsyncFunctionDef(self, node):
        self._check_handler(node)
        self.generic_visit(node)

    def _check_handler(self, node):
        for decorator in node.decorator_list:
            event_name = None
            is_sio_handler = False

            # @sio.on('event')
            if isinstance(decorator, ast.Call):
                if isinstance(decorator.func, ast.Attribute) and decorator.func.attr == "on":
                    if isinstance(decorator.func.value, ast.Name) and decorator.func.value.id == "sio":
                        is_sio_handler = True
                        if decorator.args and isinstance(decorator.args[0], ast.Constant):
                            event_name = str(decorator.args[0].value)

            # @sio.event
            elif isinstance(decorator, ast.Attribute):
                if decorator.attr == "event" and isinstance(decorator.value, ast.Name) and decorator.value.id == "sio":
                    is_sio_handler = True
                    event_name = node.name  # function name is event name, e.g. connect, disconnect

            if is_sio_handler and event_name:
                # Extract payload keys
                data_arg = "data"
                if len(node.args.args) >= 2:
                    data_arg = node.args.args[1].arg
                extractor = DataKeyExtractor(data_arg)
                extractor.visit(node)

                # Check returns (ACK payload)
                returns_ack = False
                ack_shape = None
                for sub in ast.walk(node):
                    if isinstance(sub, ast.Return) and sub.value is not None:
                        returns_ack = True
                        try:
                            ack_shape = ast.unparse(sub.value)
                        except Exception:
                            ack_shape = "Any"

                docstring = ast.get_docstring(node) or ""

                self.server_handlers[event_name] = {
                    "event": event_name,
                    "handler_func": node.name,
                    "payload_keys": sorted(list(extractor.keys)),
                    "returns_ack": returns_ack,
                    "ack_shape": ack_shape,
                    "docstring": docstring.strip(),
                    "line": node.lineno,
                }

    def visit_Call(self, node):
        # sio.emit('event', data, room=...)
        if isinstance(node.func, ast.Attribute) and node.func.attr == "emit":
            if isinstance(node.func.value, ast.Name) and node.func.value.id == "sio":
                event_name = None
                if node.args and isinstance(node.args[0], ast.Constant):
                    event_name = str(node.args[0].value)
                elif node.args:
                    try:
                        event_name = ast.unparse(node.args[0])
                    except Exception:
                        event_name = "dynamic"

                room = None
                for kw in node.keywords:
                    if kw.arg in ("room", "to"):
                        try:
                            room = ast.unparse(kw.value)
                        except Exception:
                            room = "dynamic"

                payload_str = "dynamic"
                if len(node.args) >= 2:
                    try:
                        payload_str = ast.unparse(node.args[1])
                    except Exception:
                        pass

                self.server_emits.append({
                    "event": event_name,
                    "room": room,
                    "payload_snippet": payload_str[:120],
                    "line": node.lineno,
                })

        self.generic_visit(node)


def scan_backend_python(backend_dir: Path) -> Dict[str, Any]:
    main_socket_py = backend_dir / "open_webui" / "socket" / "main.py"
    if not main_socket_py.exists():
        print(f"Error: {main_socket_py} not found")
        return {"handlers": {}, "emits": []}

    with open(main_socket_py, "r", encoding="utf-8") as f:
        code = f.read()

    tree = ast.parse(code, filename=str(main_socket_py))
    visitor = SioBackendVisitor()
    visitor.visit(tree)

    # Also scan routers and utils for sio.emit
    for py_file in backend_dir.glob("open_webui/**/*.py"):
        if py_file == main_socket_py:
            continue
        try:
            with open(py_file, "r", encoding="utf-8") as f:
                content = f.read()
            if "sio.emit" in content:
                t = ast.parse(content, filename=str(py_file))
                v = SioBackendVisitor()
                v.visit(t)
                for emit in v.server_emits:
                    emit["source_file"] = str(py_file.relative_to(backend_dir))
                    visitor.server_emits.append(emit)
        except Exception:
            pass

    return {
        "handlers": visitor.server_handlers,
        "emits": visitor.server_emits,
    }


def scan_frontend(frontend_dir: Path) -> Dict[str, Any]:
    emitted_events: Dict[str, List[Dict[str, Any]]] = {}
    listened_events: Dict[str, List[Dict[str, Any]]] = {}
    chat_event_types: Set[str] = set()

    # Regex patterns for socket calls
    emit_pattern = re.compile(r"""(?:socket|_socket|\$socket)\.emit\(\s*['"`]([^'"`]+)['"`](?:,\s*(\{.*?\}|[a-zA-Z0-9_]+))?""", re.DOTALL)
    on_pattern = re.compile(r"""(?:socket|_socket|\$socket)\.on\(\s*['"`]([^'"`]+)['"`]""")

    for path in frontend_dir.glob("**/*"):
        if path.suffix not in (".svelte", ".ts", ".js"):
            continue
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
        except Exception:
            continue

        rel_path = str(path.relative_to(frontend_dir))

        # Check emits
        for m in emit_pattern.finditer(content):
            evt = m.group(1)
            payload = m.group(2) or ""
            payload = " ".join(payload.split())[:100]
            if evt not in emitted_events:
                emitted_events[evt] = []
            emitted_events[evt].append({"file": rel_path, "payload": payload})

        # Check listeners
        for m in on_pattern.finditer(content):
            evt = m.group(1)
            if evt not in listened_events:
                listened_events[evt] = []
            listened_events[evt].append({"file": rel_path})

        # Check chatEventHandler sub-event types
        if "+layout.svelte" in rel_path:
            # Look for type === '...' or type == '...'
            for type_match in re.finditer(r"""type\s*===\s*['"]([a-zA-Z0-9_\-:]+)['"]""", content):
                chat_event_types.add(type_match.group(1))

    return {
        "client_emits": emitted_events,
        "client_listeners": listened_events,
        "chat_event_types": sorted(list(chat_event_types)),
    }


def scan_go_implementation(go_file: Path) -> Set[str]:
    implemented = set()
    if not go_file.exists():
        return implemented

    with open(go_file, "r", encoding="utf-8") as f:
        content = f.read()

    # Find switch eventName cases
    for m in re.finditer(r"""case\s+["']([^"']+)["']\s*:""", content):
        implemented.add(m.group(1))

    return implemented


def main():
    print("=" * 60)
    print("  OPENWEBUI SOCKET.IO PROTOCOL SCANNER")
    print("=" * 60)

    print("\n[1/3] Scanning Python Backend Socket.IO...")
    backend_data = scan_backend_python(BACKEND_DIR)
    print(f"  -> Found {len(backend_data['handlers'])} server event handlers (@sio.on / @sio.event)")
    print(f"  -> Found {len(backend_data['emits'])} server emit points (sio.emit)")

    print("\n[2/3] Scanning Svelte/TypeScript Frontend Socket.IO...")
    frontend_data = scan_frontend(FRONTEND_DIR)
    print(f"  -> Found {len(frontend_data['client_emits'])} client emitted events")
    print(f"  -> Found {len(frontend_data['client_listeners'])} client event listeners")
    print(f"  -> Found {len(frontend_data['chat_event_types'])} chat event types in chatEventHandler")

    print("\n[3/3] Checking Go Backend Implementation...")
    go_implemented = scan_go_implementation(GO_SOCKET_FILE)
    print(f"  -> Currently handled in Go: {sorted(list(go_implemented))}")

    # Build comprehensive protocol matrix
    all_client_to_server = sorted(list(set(backend_data["handlers"].keys()) | set(frontend_data["client_emits"].keys())))
    
    matrix = []
    for evt in all_client_to_server:
        b_info = backend_data["handlers"].get(evt, {})
        f_info = frontend_data["client_emits"].get(evt, [])

        status = "IMPLEMENTED" if evt in go_implemented else "MISSING"
        if evt in go_implemented and evt in ("user-join", "events:chat") and len(b_info.get("payload_keys", [])) > 2:
            status = "IMPLEMENTED"

        matrix.append({
            "event": evt,
            "direction": "client -> server",
            "status": status,
            "backend_payload_keys": b_info.get("payload_keys", []),
            "returns_ack": b_info.get("returns_ack", False),
            "ack_shape": b_info.get("ack_shape"),
            "client_callsites": f_info,
            "doc": b_info.get("docstring", ""),
        })

    # Server to client push events
    server_push_events = {}
    for emit in backend_data["emits"]:
        evt = emit["event"]
        if evt not in server_push_events:
            server_push_events[evt] = []
        server_push_events[evt].append(emit)

    spec = {
        "client_to_server": matrix,
        "server_to_client_emits": server_push_events,
        "client_listeners": frontend_data["client_listeners"],
        "chat_sub_event_types": frontend_data["chat_event_types"],
        "summary": {
            "total_c2s_events": len(matrix),
            "implemented": len([m for m in matrix if m["status"] == "IMPLEMENTED"]),
            "missing": len([m for m in matrix if m["status"] == "MISSING"]),
        }
    }

    with open(OUTPUT_SPEC_FILE, "w", encoding="utf-8") as f:
        json.dump(spec, f, indent=2)

    print("\n" + "=" * 60)
    print(f"  PROTOCOL SUMMARY (Saved to {OUTPUT_SPEC_FILE})")
    print("=" * 60)
    print(f"{'EVENT NAME':<25} | {'STATUS':<12} | {'ACK?':<5} | {'PAYLOAD KEYS'}")
    print("-" * 75)
    for m in matrix:
        ack_str = "YES" if m["returns_ack"] else "NO"
        keys_str = ", ".join(m["backend_payload_keys"]) if m["backend_payload_keys"] else "(none)"
        print(f"{m['event']:<25} | {m['status']:<12} | {ack_str:<5} | {keys_str}")

    print("\n" + "=" * 60)
    print("  FRONTEND 'events' CHANNEL SUB-TYPES")
    print("=" * 60)
    for t in frontend_data["chat_event_types"]:
        print(f"  - {t}")

    print("\nDone.")


if __name__ == "__main__":
    main()
