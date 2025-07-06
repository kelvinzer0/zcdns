import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { routing } from './i18n/routing'; // Import routing configuration

function Home() {
  const navigate = useNavigate();

  useEffect(() => {
    const navigatorLang = typeof navigator !== 'undefined'
      ? navigator.language.split('-')[0]
      : null;

    const targetLocale = (navigatorLang && routing.locales.includes(navigatorLang as (typeof routing.locales)[number]))
      ? navigatorLang
      : routing.defaultLocale;

    // Perform client-side redirect
    navigate(`/${targetLocale}`, { replace: true });
  }, [navigate]);

  return (
    <div>
      <p>Redirecting...</p>
    </div>
  );
}

export default Home;