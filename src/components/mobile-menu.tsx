import { Button } from "./ui/button"
import { LanguageSwitcher } from './LanguageSwitcher';
import { LocalizedLink as Link } from './LocalizedLink';
import { X } from "lucide-react"
import { cn } from "../lib/utils"
import { useTranslations } from '../lib/useTranslations';

interface MobileMenuProps {
  isOpen: boolean;
  onClose: () => void;
}

export function MobileMenu({ isOpen, onClose }: MobileMenuProps) {
  const t = useTranslations('Header');

  return (
    <>
      <div
        id="mobile-menu"
        className={cn(
          "fixed inset-y-0 left-0 w-80 bg-white z-[999999] transform transition-transform duration-300 ease-in-out md:hidden flex flex-col overflow-y-auto",
          isOpen ? "translate-x-0 shadow-2xl" : "-translate-x-full"
        )}
      >
        <div className="flex justify-end p-4">
          <button
            className="p-2  hover:bg-gray-100 transition-colors"
            onClick={onClose}
            aria-label={t('close-menu')}
          >
            <X className="h-6 w-6 text-gray-700" />
          </button>
        </div>
        <nav className="flex flex-col space-y-2 p-4 flex-1">
          <Link
            to="/"
            className="text-gray-700 hover:text-green-600 transition-colors font-medium px-2 py-2 "
            onClick={onClose}
          >
            {t('home')}
          </Link>
          <Link
            to="/dashboard"
            className="text-gray-700 hover:text-green-600 transition-colors font-medium px-2 py-2 "
            onClick={onClose}
          >
            {t('dashboard')}
          </Link>
          <Link
            to="/grants"
            className="text-gray-700 hover:text-green-600 transition-colors font-medium px-2 py-2 "
            onClick={onClose}
          >
            {t('grants')}
          </Link>
          <Link
            to="/abuse-report"
            className="text-gray-700 hover:text-green-600 transition-colors font-medium px-2 py-2 "
            onClick={onClose}
          >
            {t('abuse-report')}
          </Link>
          <a
            target="_blank"
            href="https://codeberg.org/zcdns/mediakit/archive/main.tar.gz"
            className="text-gray-700 hover:text-green-600 transition-colors font-medium px-2 py-2 "
            onClick={onClose}
          >
            {t('media-kit')}
          </a>
          <div className="mt-auto pt-4 border-t border-gray-200">
            <LanguageSwitcher />
            <Button asChild className="bg-[#012241] hover:bg-[#02365f] text-white w-full mt-4">
              <Link to="https://www.zcdns.id/dashboard" target="_blank" onClick={onClose}>
                {t('get-started-free')}
              </Link>
            </Button>
          </div>
        </nav>
      </div>

      {/* Mobile Menu Overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-black/50 backdrop-blur-sm z-[999998] md:hidden"
          onClick={onClose}
        ></div>
      )}
    </>
  )
}
