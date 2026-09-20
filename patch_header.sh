#!/bin/bash
cat << 'INNER_EOF' > tmp_header.js
const fs = require('fs');
let code = fs.readFileSync('src/components/header.tsx', 'utf8');

// Add dashboardApi import if missing
if (!code.includes('dashboardApi')) {
    code = code.replace("import { useTranslations } from '../lib/useTranslations';", "import { useTranslations } from '../lib/useTranslations';\nimport { dashboardApi } from './dashboard/api';");
}

const handleGlobalLogout = `  const handleGlobalLogout = async () => {
    try {
      await dashboardApi.deleteSession();
    } catch {}
    localStorage.removeItem("zcdns_admin_token");
    document.cookie = "zcdns_admin_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    document.cookie = "zcdns_subdomain=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    window.location.href = '/';
  };
`;

code = code.replace('  const isDocsPage = pathname.includes(\'/docs\');', '  const isDocsPage = pathname.includes(\'/docs\');\n  const isAppPage = pathname.includes(\'/dashboard\') || pathname.includes(\'/admin\');\n' + handleGlobalLogout);

const oldButton = `<Button
                asChild
                className="bg-[#012241] hover:bg-[#02365f] text-white shadow-lg hover:shadow-xl transition-all duration-200"
              >
                <Link to="/dashboard">{t('get-started-free')}</Link>
              </Button>`;

const newButton = `{isAppPage ? (
              <Button
                onClick={handleGlobalLogout}
                className="bg-red-600 hover:bg-red-700 text-white shadow-lg hover:shadow-xl transition-all duration-200"
              >
                Logout
              </Button>
            ) : (
              <Button
                asChild
                className="bg-[#012241] hover:bg-[#02365f] text-white shadow-lg hover:shadow-xl transition-all duration-200"
              >
                <Link to="/dashboard">{t('get-started-free')}</Link>
              </Button>
            )}`;

code = code.replace(oldButton, newButton);

fs.writeFileSync('src/components/header.tsx', code);
INNER_EOF
node tmp_header.js || node tmp_header.cjs || mv tmp_header.js tmp_header.cjs && node tmp_header.cjs
