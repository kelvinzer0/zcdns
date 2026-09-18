import { Link } from 'react-router-dom';
import { useTranslations } from '../lib/useTranslations';
import { useEffect, useState } from 'react';

export function Footer() {
  const t = useTranslations('Footer');
  const [pingData, setPingData] = useState<{ status: 'loading' | 'online' | 'offline', latency: number | null }>({ status: 'loading', latency: null });

  useEffect(() => {
    let isMounted = true;
    const pingAPI = async () => {
      try {
        const start = performance.now();
        const res = await fetch('/api/health');
        if (res.ok) {
          const end = performance.now();
          if (isMounted) {
            setPingData({ status: 'online', latency: Math.round(end - start) });
          }
        } else {
          if (isMounted) setPingData({ status: 'offline', latency: null });
        }
      } catch (e) {
        if (isMounted) setPingData({ status: 'offline', latency: null });
      }
    };
    pingAPI();
    
    // Periodically check every 60 seconds
    const interval = setInterval(pingAPI, 60000);
    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, []);

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
                  src="/zcdns-light-logo.svg"
                  alt={t('zerocentdns-logo')}
                  className="transition-transform duration-300 group-hover:rotate-12 h-24 w-auto"
                />
              </Link>

              <p className="text-gray-300 leading-relaxed max-w-2xl mx-auto text-lg italic">
                {t('description')}
              </p>

              {/* Open Source Info */}
              <div className="pt-2 flex justify-center">
                <a 
                  href="https://github.com/kelvinzer0/zcdns" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2.5 px-5 py-2.5 rounded-full bg-gray-800/50 border border-gray-700/50 hover:bg-gray-700 hover:border-gray-500 text-gray-300 hover:text-white transition-all duration-300 backdrop-blur-sm group shadow-sm hover:shadow-md"
                >
                  <svg className="w-5 h-5 text-gray-400 group-hover:text-white transition-colors" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.285 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                  <span className="text-sm font-medium tracking-wide">Proudly Open Source on GitHub</span>
                </a>
              </div>
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
              {pingData.status === 'online' ? (
                <>
                  <div className="w-2 h-2 bg-green-500 rounded-full shadow-[0_0_8px_rgba(34,197,94,0.6)]"></div>
                  <span>{t('service-active')} ({pingData.latency}ms)</span>
                </>
              ) : pingData.status === 'offline' ? (
                <>
                  <div className="w-2 h-2 bg-red-500 rounded-full"></div>
                  <span className="text-red-400">Service Degraded</span>
                </>
              ) : (
                <>
                  <div className="w-2 h-2 bg-yellow-500 rounded-full animate-pulse"></div>
                  <span>Checking...</span>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </footer>
  )
}
