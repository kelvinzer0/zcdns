import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { useLocation, useNavigate } from "react-router-dom";
import { Languages } from "lucide-react";

import { routing } from "../i18n/routing";
import { useLocale } from "../i18n/I18nProvider";

export function LanguageSwitcher() {
  const locale = useLocale();
  const navigate = useNavigate();
  const location = useLocation();

  const handleLocaleChange = (newLocale: string) => {
    const pathSegments = location.pathname.split('/');
    pathSegments[1] = newLocale;
    const newPath = pathSegments.join('/');
    navigate(newPath);
  };

  // Map locale codes to display names
  const localeDisplayNames: Record<string, string> = {
    en: "English",
    id: "Indonesia",
  };

  return (
    <Select onValueChange={handleLocaleChange} value={locale}>
      <SelectTrigger className="w-[130px] border-blue-100 rounded-xs">
        <div className="flex items-center gap-2">
          <Languages className="w-4 h-4" />
          <SelectValue placeholder="Language" />
        </div>
      </SelectTrigger>
      <SelectContent>
        {routing.locales.map((loc) => (
          <SelectItem key={loc} value={loc}>
            {localeDisplayNames[loc] || loc.toUpperCase()}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}