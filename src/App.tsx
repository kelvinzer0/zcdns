import { Routes, Route, useLocation, Navigate } from 'react-router-dom';
import { LocalizedLayout } from './components/LocalizedLayout';
import { routing } from './i18n/routing';
import './App.css';
import { I18nProvider } from './i18n/I18nProvider';

function App() {
  const location = useLocation();
  const currentLocale = location.pathname.split('/')[1] as (typeof routing.locales)[number] || routing.defaultLocale;

  // Redirect from '/' to '/:defaultLocale' or '/:browserLocale'
  if (location.pathname === '/') {
    const navigatorLang = typeof navigator !== 'undefined' ? navigator.language.split('-')[0] : null;
    const targetLocale = (navigatorLang && routing.locales.includes(navigatorLang as (typeof routing.locales)[number]))
      ? navigatorLang
      : routing.defaultLocale;
    return <Navigate to={`/${targetLocale}`} replace />;
  }

  // If the locale in the URL is not valid, redirect to default locale with the same path
  if (currentLocale && !routing.locales.includes(currentLocale as (typeof routing.locales)[number])) {
    return <Navigate to={`/${routing.defaultLocale}${location.pathname.substring(currentLocale.length + 1)}`} replace />;
  }

  return (
    <I18nProvider locale={currentLocale}>
      <Routes>
        <Route path="/:locale/*" element={<LocalizedLayout />} />
        {/* Fallback for any other unmatched paths, redirect to default locale */}
        <Route path="*" element={<Navigate to={`/${routing.defaultLocale}`} replace />} />
      </Routes>
    </I18nProvider>
  );
}

export default App;
