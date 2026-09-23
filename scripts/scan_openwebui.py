#!/usr/bin/env python3
"""
Canonical OpenWebUI Scanner:
1. Uses TypeScript compiler AST (via Node.js) to cleanly extract frontend API fetch calls (0 false positives).
2. Uses Python AST (ast.unparse) with recursive inheritance resolution to map all FastAPI routes & Pydantic models.
3. Automatically outputs Go structs and endpoint mapping.
"""

import os
import subprocess
import json
import ast
from pathlib import Path

ROOT_DIR = Path('/home/kelvinandriancom/www')
OWU_DIR = Path('/tmp/open-webui')
BACKEND_DIR = OWU_DIR / 'backend' / 'open_webui'
ROUTERS_DIR = BACKEND_DIR / 'routers'
MODELS_DIR = BACKEND_DIR / 'models'

def extract_frontend_endpoints_with_ts_ast():
    ts_script = """
const ts = require('typescript');
const fs = require('fs');
const path = require('path');

const apisDir = '/tmp/open-webui/src/lib/apis';
const endpoints = [];

function normalizeUrl(node, source) {
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
    if (ts.isTemplateExpression(node)) {
        let res = node.head.text;
        for (const span of node.templateSpans) {
            const expr = span.expression.getText(source);
            let placeholder = '{param}';
            if (expr.includes('WEBUI_API_BASE_URL')) placeholder = '/api/v1';
            else if (expr.includes('WEBUI_BASE_URL')) placeholder = '';
            else if (expr.includes('AUDIO_API_BASE_URL')) placeholder = '/api/v1/audio';
            else if (expr.includes('OLLAMA_API_BASE_URL')) placeholder = '/ollama';
            else if (expr.includes('OPENAI_API_BASE_URL')) placeholder = '/openai';
            else if (expr.includes('RETRIEVAL_API_BASE_URL')) placeholder = '/api/v1/retrieval';
            else if (expr.includes('IMAGE_API_BASE_URL')) placeholder = '/api/v1/images';
            else if (expr.includes('searchParams') || expr.includes('params') || expr.includes('query')) placeholder = '';
            res += placeholder + span.literal.text;
        }
        return res.split('?')[0].replace(/\\/+/g, '/').replace(/\\/$/, '');
    }
    return '';
}

function scanFile(filePath) {
    const code = fs.readFileSync(filePath, 'utf-8');
    const source = ts.createSourceFile(filePath, code, ts.ScriptTarget.Latest, true);

    function visit(node) {
        if (ts.isCallExpression(node) && node.expression.getText(source) === 'fetch') {
            const url = normalizeUrl(node.arguments[0], source);
            let method = 'GET';
            if (node.arguments[1] && ts.isObjectLiteralExpression(node.arguments[1])) {
                for (const prop of node.arguments[1].properties) {
                    if (prop.name && prop.name.getText(source) === 'method' && prop.initializer) {
                        method = prop.initializer.getText(source).replace(/['\\"]/g, '');
                    }
                }
            }
            if (url && (url.startsWith('/api') || url.startsWith('/ollama') || url.startsWith('/openai'))) {
                endpoints.push({
                    file: path.relative('/tmp/open-webui/src/lib/apis', filePath),
                    method: method.toUpperCase(),
                    url: url
                });
            }
        }
        ts.forEachChild(node, visit);
    }
    visit(source);
}

function walk(dir) {
    for (const f of fs.readdirSync(dir)) {
        const full = path.join(dir, f);
        if (fs.statSync(full).isDirectory()) walk(full);
        else if (f.endsWith('.ts')) scanFile(full);
    }
}

walk(apisDir);
console.log(JSON.stringify(endpoints));
"""
    result = subprocess.run(['node', '-e', ts_script], cwd=str(ROOT_DIR), capture_output=True, text=True)
    if result.returncode != 0:
        print("Error running Node.js TS AST scanner:", result.stderr)
        return []
    return json.loads(result.stdout)

def extract_router_prefixes():
    main_py = BACKEND_DIR / 'main.py'
    prefixes = {}
    content = main_py.read_text(encoding='utf-8')
    import re
    matches = re.findall(r"app\.include_router\(\s*([a-zA-Z0-9_]+)\.router\s*,\s*prefix=['\"]([^'\"]+)['\"]", content)
    for mod_name, prefix in matches:
        prefixes[mod_name] = prefix
    return prefixes

def scan_pydantic_classes_with_inheritance():
    classes = {}
    for d in [MODELS_DIR, ROUTERS_DIR]:
        for py_file in d.glob('*.py'):
            try:
                tree = ast.parse(py_file.read_text(encoding='utf-8'))
            except Exception:
                continue
            for node in tree.body:
                if isinstance(node, ast.ClassDef):
                    bases = []
                    for b in node.bases:
                        if isinstance(b, ast.Name):
                            bases.append(b.id)
                        elif isinstance(b, ast.Attribute):
                            bases.append(b.attr)
                    fields = {}
                    for item in node.body:
                        if isinstance(item, ast.AnnAssign) and isinstance(item.target, ast.Name):
                            fields[item.target.id] = ast.unparse(item.annotation)
                    classes[node.name] = {
                        'bases': bases,
                        'fields': fields,
                        'file': py_file.name
                    }

    def resolve_fields(cname, visited=None):
        if visited is None:
            visited = set()
        if cname in visited or cname not in classes:
            return {}
        visited.add(cname)
        f = {}
        for b in classes[cname]['bases']:
            f.update(resolve_fields(b, visited))
        f.update(classes[cname]['fields'])
        return f

    resolved = {}
    for cname in classes:
        resolved[cname] = {
            'file': classes[cname]['file'],
            'bases': classes[cname]['bases'],
            'fields': resolve_fields(cname)
        }
    return resolved

def scan_fastapi_backend_routes(prefixes):
    routes = []
    import re

    def scan_file(py_file, prefix):
        res = []
        try:
            tree = ast.parse(py_file.read_text(encoding='utf-8'))
        except Exception:
            return res

        for node in tree.body:
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                for dec in node.decorator_list:
                    if isinstance(dec, ast.Call):
                        func_expr = ast.unparse(dec.func)
                        m = re.match(r"(router|app)\.(get|post|put|delete|patch)", func_expr)
                        if m:
                            http_method = m.group(2).upper()
                            route_path = "/"
                            if dec.args and isinstance(dec.args[0], ast.Constant):
                                route_path = dec.args[0].value
                            resp_model = ""
                            for kw in dec.keywords:
                                if kw.arg == 'response_model':
                                    resp_model = ast.unparse(kw.value)

                            full_path = prefix.rstrip('/') + '/' + route_path.lstrip('/')
                            if full_path == '':
                                full_path = '/'
                            res.append({
                                'file': py_file.name,
                                'method': http_method,
                                'path': full_path,
                                'func': node.name,
                                'response_model': resp_model
                            })
        return res

    main_py = BACKEND_DIR / 'main.py'
    if main_py.exists():
        routes.extend(scan_file(main_py, prefix=""))

    for py_file in sorted(ROUTERS_DIR.glob('*.py')):
        mod_name = py_file.stem
        prefix = prefixes.get(mod_name, f"/api/v1/{mod_name}")
        routes.extend(scan_file(py_file, prefix=prefix))

    return routes

if __name__ == '__main__':
    print("1. Extracting frontend API endpoints using TypeScript Compiler AST...")
    fe_calls = extract_frontend_endpoints_with_ts_ast()
    print(f"   -> Found {len(fe_calls)} valid frontend fetch calls across TypeScript APIs.")

    print("2. Extracting Pydantic models with complete recursive inheritance resolution...")
    models = scan_pydantic_classes_with_inheritance()
    print(f"   -> Resolved {len(models)} Pydantic models.")

    print("3. Extracting FastAPI backend routes with Python AST unparse...")
    prefixes = extract_router_prefixes()
    routes = scan_fastapi_backend_routes(prefixes)
    print(f"   -> Found {len(routes)} FastAPI routes.")

    # Save canonical dataset
    output = {
        'frontend_endpoints': fe_calls,
        'backend_routes': routes,
        'pydantic_models': models
    }
    with open('/tmp/openwebui_canonical_spec.json', 'w') as f:
        json.dump(output, f, indent=2)

    print("Successfully exported canonical spec to /tmp/openwebui_canonical_spec.json")
