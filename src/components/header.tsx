import { useEffect, useState } from "react"
import { useLocation } from "react-router-dom"

import { Button } from "./ui/button"
import { LanguageSwitcher } from './LanguageSwitcher';
import { LocalizedLink as Link } from './LocalizedLink';
import { Menu } from "lucide-react"
import { WarningBanner } from './WarningBanner';
import { cn } from "../lib/utils"
import { useTranslations } from '../lib/useTranslations';
import { dashboardApi } from './dashboard/api';

export function Header({ setMobileMenuOpen }: { setMobileMenuOpen: (isOpen: boolean) => void }) {
  const t = useTranslations('Header');
  const [isScrolled, setIsScrolled] = useState(false)
  const { pathname } = useLocation();
  const isDocsPage = pathname.includes('/docs');
  const isAppPage = pathname.includes('/dashboard') || pathname.includes('/admin');

  const handleGlobalLogout = async () => {
    try {
      await dashboardApi.deleteSession();
    } catch {}
    localStorage.removeItem("zcdns_admin_token");
    document.cookie = "zcdns_admin_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    document.cookie = "zcdns_subdomain=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    window.location.href = '/';
  };

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 10)
    }
    window.addEventListener("scroll", handleScroll)
    return () => window.removeEventListener("scroll", handleScroll)
  }, [])

  return (
      <header
        className={cn(
          "fixed top-0 left-0 right-0 z-50 transition-all duration-300",
          isDocsPage 
            ? "bg-white" 
            : isScrolled 
              ? "bg-white/80 backdrop-blur-md border-b border-white/20 shadow-lg shadow-black/5" 
              : "bg-transparent",
        )}
      >
        <WarningBanner />
        <div className="container mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Logo */}
            <Link to="/" className="inline-block group">
              <img
                src="/zcdns-dark-logo.svg"
                alt={t('zerocentdns-logo')}
                className="transition-transform duration-300 group-hover:scale-105 h-12 w-auto mx-2"
              />
            </Link>

            {/* Desktop Navigation */}
            <nav className="hidden md:flex items-center space-x-8">
              <Link to="/" className="text-gray-700 hover:text-green-600 transition-colors font-medium">
                {t('home')}
              </Link>
              <Link to="/dashboard" className="text-gray-700 hover:text-green-600 transition-colors font-medium">
                {t('dashboard')}
              </Link>
              <Link to="/abuse-report" className="text-gray-700 hover:text-green-600 transition-colors font-medium">
                {t('abuse-report')}
              </Link>
              <a target="_blank" href="https://codeberg.org/zcdns/mediakit/archive/main.tar.gz" className="text-gray-700 hover:text-green-600 transition-colors font-medium">
                {t('media-kit')}
              </a>
            </nav>

            {/* CTA Button */}
            <div className="hidden md:flex items-center space-x-4">
              <LanguageSwitcher />
              {isAppPage ? (
                <Button
                  onClick={handleGlobalLogout}
                  className="bg-red-600 hover:bg-red-700 text-white shadow-lg hover:shadow-xl transition-all duration-200 cursor-pointer"
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
              )}
            </div>

            {/* Mobile Menu Button */}
            <button
              className="md:hidden p-2  hover:bg-gray-100 transition-colors"
              onClick={() => setMobileMenuOpen(true)}
              aria-label={t('open-menu')}
            >
              <Menu className="h-6 w-6 text-gray-700" />
            </button>
          </div>
        </div>
      </header>
  )
}

