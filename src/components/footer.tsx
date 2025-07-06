import { Link } from 'react-router-dom'; // Changed to react-router-dom Link
import { useTranslations } from '../lib/useTranslations'; // Adjusted import path for placeholder

export function Footer() {
  const t = useTranslations('Footer');
  return (
    <footer className="bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 text-white">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8">
        {/* Main Footer Content */}
        <div className="py-12 lg:py-16">
          <div className="text-center space-y-8">
            {/* Brand Section */}
            <div className="space-y-6">
              <Link to="/" className="inline-flex items-center group relative w-max">
                <img
                  src="/zcdns-light-logo.svg"  // Replaced next/image with img tag, using a placeholder for now
                  alt={t('zerocentdns-logo')}
                  className="transition-transform duration-300 group-hover:rotate-12 h-24 w-auto"
                />
              </Link>

              <p className="text-gray-300 leading-relaxed max-w-2xl mx-auto text-lg italic">
                {t('description')}
              </p>
            </div>


          </div>
        </div>

        {/* Bottom Bar */}
        <div className="border-t border-gray-700/50 py-3">
          <div className="flex justify-between items-center text-sm text-gray-400">
            <div className="flex items-center gap-2">
              <span>Copyright © {new Date().getFullYear()}</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 bg-green-500  animate-pulse rounded-full"></div>
              <span>{t('service-active')}</span> 
            </div>
            
          </div>
        </div>
      </div>
    </footer>
  )
}
