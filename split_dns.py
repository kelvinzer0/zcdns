import re
import os

with open("backend/dns/server.go", "r") as f:
    content = f.read()

# We need to extract the imports manually or let goimports do it
# but we can just copy the whole import block to every file and let goimports clean it up!

import_block_match = re.search(r'(?s)import \((.*?)\)', content)
imports = import_block_match.group(0) if import_block_match else ""

def create_file(filename, funcs):
    with open(filename, "w") as f:
        f.write("package dns\n\n" + imports + "\n\n")
        
        for func_name in funcs:
            # Match func (s *Server) func_name(...) OR func func_name(...)
            # capturing until the next func or end of file
            # This regex is a bit tricky, let's use a simpler approach:
            pass

# Actually, doing it via a script might miss types or consts.
