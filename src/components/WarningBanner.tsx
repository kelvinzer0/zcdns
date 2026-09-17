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
    <div className="sticky top-0 z-50 bg-red-100 border-b-2 border-red-200 text-red-900">
      <div className="container mx-auto flex items-center justify-between gap-4 p-3">
        <div className="flex flex-1 items-center gap-3">
          <AlertTriangle className="h-5 w-5 flex-shrink-0 self-center text-red-500" />
          <p className="text-sm leading-normal self-center">
            {t('disclaimer_part1')}
            <Link to={`/${locale}/abuse-report`} className="font-semibold underline hover:text-red-950">
              {t('disclaimer_link')}
            </Link>
            .
          </p>
        </div>
        <button
          onClick={handleClose}
          className="flex h-7 w-7 flex-shrink-0 items-center justify-center self-center rounded hover:bg-red-200 focus:outline-none focus:ring-2 focus:ring-red-600"
          aria-label={t('close_button_aria_label')}
        >
          <X className="h-5 w-5" />
        </button>
      </div>
    </div>
  );
}
