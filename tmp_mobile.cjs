const fs = require('fs');
let code = fs.readFileSync('src/components/mobile-menu.tsx', 'utf8');

if (!code.includes('dashboardApi')) {
    code = code.replace("import { useTranslations } from '../lib/useTranslations';", "import { useTranslations } from '../lib/useTranslations';\nimport { dashboardApi } from './dashboard/api';\nimport { useLocation } from 'react-router-dom';");
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

if (!code.includes('handleGlobalLogout')) {
    code = code.replace("export function MobileMenu({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {\n  const t = useTranslations('Header');", "export function MobileMenu({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {\n  const t = useTranslations('Header');\n  const { pathname } = useLocation();\n  const isAppPage = pathname.includes('/dashboard') || pathname.includes('/admin');\n" + handleGlobalLogout);
}

const oldButton = `<Button asChild className="bg-[#012241] hover:bg-[#02365f] text-white w-full mt-4">
              <Link to="https://www.zcdns.id/dashboard" target="_blank" onClick={onClose}>
                {t('get-started-free')}
              </Link>
            </Button>`;

const newButton = `{isAppPage ? (
              <Button onClick={handleGlobalLogout} className="bg-red-600 hover:bg-red-700 text-white w-full mt-4">
                Logout
              </Button>
            ) : (
              <Button asChild className="bg-[#012241] hover:bg-[#02365f] text-white w-full mt-4">
                <Link to="https://www.zcdns.id/dashboard" target="_blank" onClick={onClose}>
                  {t('get-started-free')}
                </Link>
              </Button>
            )}`;

code = code.replace(oldButton, newButton);
fs.writeFileSync('src/components/mobile-menu.tsx', code);
