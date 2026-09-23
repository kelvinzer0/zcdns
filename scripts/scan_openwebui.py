import os
import re
import ast
import json
from pathlib import Path
from collections import defaultdict

OWU_DIR = Path('/tmp/open-webui')
BACKEND_DIR = OWU_DIR / 'backend' / 'open_webui'
ROUTERS_DIR = BACKEND_DIR / 'routers'
MODELS_DIR = BACKEND_DIR / 'models'
SRC_DIR = OWU_DIR / 'src'
GO_API_DIR = Path('/home/kelvinandriancom/www/backend/api')

def extract_router_prefixes():
    main_py = BACKEND_DIR / 'main.py'
    prefixes = {}
    content = main_py.read_text(encoding='utf-8')
    # Match patterns like: app.include_router(chats.router, prefix='/api/v1/chats'
    matches = re.findall(r"app\.include_router\(\s*([a-zA-Z0-9_]+)\.router\s*,\s*prefix=['\"]([^'\"]+)['\"]", content)
    for mod_name, prefix in matches:
        prefixes[mod_name] = prefix
    return prefixes

def get_ast_name(node):
    if node is None:
        return ""
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Constant):
        return str(node.value)
    if isinstance(node, ast.Attribute):
        return f"{get_ast_name(node.value)}.{node.attr}"
    if isinstance(node, ast.Subscript):
        return f"{get_ast_name(node.value)}[{get_ast_name(node.slice)}]"
    if isinstance(node, ast.BinOp) and isinstance(node.op, ast.BitOr):
        return f"{get_ast_name(node.left)} | {get_ast_name(node.right)}"
    if isinstance(node, ast.Tuple):
        return ", ".join(get_ast_name(e) for e in node.elts)
    return ""

def scan_backend_routes(prefixes):
    routes = []
    
    # Also scan main.py directly for @app.get etc.
    main_py = BACKEND_DIR / 'main.py'
    if main_py.exists():
        routes.extend(scan_file_routes(main_py, prefix=""))

    for py_file in sorted(ROUTERS_DIR.glob('*.py')):
        mod_name = py_file.stem
        prefix = prefixes.get(mod_name, f"/api/v1/{mod_name}")
        routes.extend(scan_file_routes(py_file, prefix=prefix))
        
    return routes

def scan_file_routes(py_file, prefix):
    routes = []
    try:
        tree = ast.parse(py_file.read_text(encoding='utf-8'))
    except Exception as e:
        return routes

    for node in tree.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            for dec in node.decorator_list:
                # e.g. @router.get('/...', response_model=...) or @app.get(...)
                if isinstance(dec, ast.Call):
                    func_expr = get_ast_name(dec.func)
                    m = re.match(r"(router|app)\.(get|post|put|delete|patch)", func_expr)
                    if m:
                        http_method = m.group(2).upper()
                        route_path = "/"
                        if dec.args:
                            first_arg = dec.args[0]
                            if isinstance(first_arg, ast.Constant):
                                route_path = first_arg.value
                        
                        resp_model = ""
                        for kw in dec.keywords:
                            if kw.arg == 'response_model':
                                resp_model = get_ast_name(kw.value)
                                
                        # Request body detection: look for args not in (request, response, user, db, background_tasks)
                        req_body_type = ""
                        for arg in node.args.args:
                            arg_name = arg.arg
                            if arg_name in ('request', 'response', 'user', 'db', 'background_tasks', 'auth_token'):
                                continue
                            if arg.annotation:
                                ann = get_ast_name(arg.annotation)
                                # Exclude query/header primitives unless they are models
                                if not any(x in ann.lower() for x in ('query', 'header', 'depends', 'int', 'bool', 'str', 'request')):
                                    req_body_type = f"{arg_name}: {ann}"

                        full_path = prefix.rstrip('/') + '/' + route_path.lstrip('/')
                        if full_path == '':
                            full_path = '/'
                        routes.append({
                            'file': py_file.name,
                            'method': http_method,
                            'path': full_path,
                            'func': node.name,
                            'req_body': req_body_type,
                            'response_model': resp_model
                        })
    return routes

def scan_pydantic_models():
    models = {}
    scan_dirs = [MODELS_DIR, ROUTERS_DIR]
    for d in scan_dirs:
        for py_file in d.glob('*.py'):
            try:
                tree = ast.parse(py_file.read_text(encoding='utf-8'))
            except Exception:
                continue
            for node in tree.body:
                if isinstance(node, ast.ClassDef):
                    # Check if inherits from BaseModel
                    base_names = [get_ast_name(b) for b in node.bases]
                    fields = {}
                    for item in node.body:
                        if isinstance(item, ast.AnnAssign) and isinstance(item.target, ast.Name):
                            field_name = item.target.id
                            field_type = get_ast_name(item.annotation)
                            default_val = None
                            if item.value:
                                if isinstance(item.value, ast.Constant):
                                    default_val = item.value.value
                                elif isinstance(item.value, ast.List):
                                    default_val = []
                                elif isinstance(item.value, ast.Dict):
                                    default_val = {}
                            fields[field_name] = {
                                'type': field_type,
                                'default': default_val
                            }
                    if fields:
                        models[node.name] = {
                            'file': py_file.name,
                            'bases': base_names,
                            'fields': fields
                        }
    return models

def scan_frontend_calls():
    # Scan src/ for all fetch(`${WEBUI_API_BASE_URL}/...`) or fetch(`/api/...`)
    calls = []
    pattern = re.compile(r"fetch\(\s*`?(\$?[{a-zA-Z0-9_}]+[/\w\${}?:=&\-\.]*)`?\s*,?\s*(\{[^;]*?\})?\)", re.MULTILINE | re.DOTALL)
    
    for f in SRC_DIR.glob('**/*'):
        if f.suffix in ('.ts', '.svelte', '.js'):
            content = f.read_text(encoding='utf-8', errors='ignore')
            for m in re.finditer(r"fetch\(([^,\)]+)(?:,\s*(\{.*?\})\s*)?\)", content, re.DOTALL):
                raw_url = m.group(1).strip().strip("'`\"")
                raw_opts = m.group(2) or ""
                method = "GET"
                method_m = re.search(r"method:\s*['\"]([A-Z]+)['\"]", raw_opts)
                if method_m:
                    method = method_m.group(1)
                
                # Normalize url
                clean_url = raw_url.replace('${WEBUI_API_BASE_URL}', '/api/v1')
                clean_url = clean_url.replace('${WEBUI_BASE_URL}', '')
                clean_url = clean_url.replace('$WEBUI_API_BASE_URL', '/api/v1')
                clean_url = clean_url.replace('$WEBUI_BASE_URL', '')
                clean_url = re.sub(r"\?.*$", "", clean_url) # remove query
                clean_url = re.sub(r"\$\{[^}]+\}", "{param}", clean_url)
                
                if '/api' in clean_url or clean_url.startswith('/'):
                    calls.append({
                        'file': f.relative_to(OWU_DIR).as_posix(),
                        'method': method,
                        'url': clean_url
                    })
    return calls

if __name__ == '__main__':
    prefixes = extract_router_prefixes()
    routes = scan_backend_routes(prefixes)
    models = scan_pydantic_models()
    frontend_calls = scan_frontend_calls()

    print(f"Total Backend Routes scanned: {len(routes)}")
    print(f"Total Pydantic Models scanned: {len(models)}")
    print(f"Total Frontend API calls scanned: {len(frontend_calls)}")
    
    # Save to JSON for analysis
    output = {
        'routes': routes,
        'models': models,
        'frontend_calls': frontend_calls
    }
    with open('/tmp/openwebui_analysis.json', 'w') as f:
        json.dump(output, f, indent=2)
    print("Saved full analysis to /tmp/openwebui_analysis.json")
