#!/bin/bash

# Replace the localStorage block with a proper dashboardApi call
cat << 'INNER' > tmp_sed.js
const fs = require('fs');
let code = fs.readFileSync('src/components/VaultAuthPage.tsx', 'utf8');

// Add import for dashboardApi
if (!code.includes('dashboardApi')) {
    code = code.replace("import { Button } from './ui/button';", "import { Button } from './ui/button';\nimport { dashboardApi } from './dashboard/api';");
}

// Replace the useEffect
const oldEffect = `  // Load existing session subdomain from localStorage if available
  useEffect(() => {
    try {
      const savedSession = localStorage.getItem('zcdns_session');
      if (savedSession) {
        const parsed = JSON.parse(savedSession);
        if (parsed?.subdomain) {
          setSubdomain(parsed.subdomain);
        }
      }
    } catch {
      // ignore
    }
  }, []);`;

const newEffect = `  // Load existing session from backend
  useEffect(() => {
    dashboardApi.getSession().then((session) => {
      if (session && session.subdomain) {
        setSubdomain(session.subdomain);
      }
    }).catch(() => {});
  }, []);`;

code = code.replace(oldEffect, newEffect);

fs.writeFileSync('src/components/VaultAuthPage.tsx', code);
INNER
node tmp_sed.js
