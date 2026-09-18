import { useState, useEffect } from 'react';
import { X, AlertTriangle } from 'lucide-react';
import { useTranslations } from '../lib/useTranslations';
import { Link, useParams } from 'react-router-dom';

export function WarningBanner() {
  const t = useTranslations('WarningBanner');
  const { locale } = useParams<{ locale: string }>();
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const isClosed = localStorage.getItem('warningBannerClosed');
    if (!isClosed) {
      setIsVisible(true);
    }
  }, []);

  const handleClose = () => {
    setIsVisible(false);
    localStorage.setItem('warningBannerClosed', 'true');
  };

  if (!isVisible) {
    return null;
  }

  return (
    <div className="sticky top-0 z-50 bg-rose-50/80 backdrop-blur-xl border-b border-rose-100/60 shadow-[0_1px_3px_rgba(0,0,0,0.02)] transition-all duration-300">
      <div className="container mx-auto max-w-7xl flex items-center justify-between gap-4 py-3 sm:py-3.5 px-4 sm:px-6">
        <div className="flex flex-1 items-center gap-3 sm:gap-4">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white/70 shadow-sm border border-rose-100">
            <AlertTriangle className="h-4 w-4 text-rose-500" strokeWidth={2.5} />
          </div>
          <p className="text-[13px] sm:text-sm font-medium leading-relaxed mb-0 text-rose-900/90">
            {t('disclaimer_part1')}
            <Link to={`/${locale}/abuse-report`} className="font-semibold text-rose-950 decoration-rose-300 underline underline-offset-4 hover:decoration-rose-600 transition-colors">
              {t('disclaimer_link')}
            </Link>
            .
          </p>
        </div>
        <button
          onClick={handleClose}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-rose-400 hover:text-rose-800 hover:bg-rose-200/50 transition-all focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-1 focus:ring-offset-rose-50"
          aria-label={t('close_button_aria_label')}
        >
          <X className="h-4 w-4" strokeWidth={2.5} />
        </button>
      </div>
    </div>
  );
}
