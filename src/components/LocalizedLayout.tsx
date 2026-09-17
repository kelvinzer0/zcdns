import { Navigate, Route, Routes, useLocation, useParams } from 'react-router-dom';

import { About } from './about';
import { AbusePage } from './abuse-page';
import { DashboardPage } from './dashboard/DashboardPage';
import { DocPageWrapper } from './doc-page-wrapper';
import { EmailServiceOffer } from './EmailServiceOffer';
import { Features } from './features';
import { Footer } from './footer';
import { GrantsPage } from './GrantsPage'; // Import the GrantsPage component
import { Header } from './header';
import { Hero } from './hero';
import { MobileMenu } from './mobile-menu';
import { Partners } from './partners';
import { Seo } from './Seo'; // Import the Seo component
import { Statistics } from './Statistics'; // Import the Statistics component
import { routing } from '../i18n/routing';
import { useState } from 'react';

// JSON-LD for the organization
const organizationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  'name': 'ZeroCentDNS',
  'url': 'https://www.zcdns.id',
  'logo': 'https://www.zcdns.id/zcdns-light-logo.svg', // Pastikan logo ini ada
  'sameAs': [
    // Tambahkan link media sosial Anda di sini
    // 'https://twitter.com/yourhandle',
    // 'https://www.linkedin.com/company/yourcompany'
  ],
  'contactPoint': {
    '@type': 'ContactPoint',
    'contactType': 'customer support',
    'email': 'support@zcdns.id', // Ganti dengan email support Anda
  },
};

export function LocalizedLayout() {
  const { locale } = useParams<{ locale: string }>();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  if (!locale || !routing.locales.includes(locale as (typeof routing.locales)[number])) {
    return <Navigate to={`/${routing.defaultLocale}`} replace />;
  }

  // Tentukan data SEO berdasarkan halaman
  const getSeoData = () => {
    const path = location.pathname.replace(`/${locale}`, '') || '/';
    switch (path) {
      case '/about':
        return { title: 'About Us', description: 'Learn more about the mission and team behind ZCDNS.' };
      case '/abuse-report':
        return { title: 'Abuse Report', description: 'Report abuse or malicious activity related to our DNS services.' };
      case '/grants':
        return { title: 'Grants & Donations', description: 'Help us sustain and grow the ZCDNS ecosystem.' };
      case '/dashboard':
        return { title: 'DNS Playground & Dashboard - ZeroCentDNS', description: 'Interactive DNS sandbox with live query stream and full record management.' };
      default:
        // Halaman utama
        return { 
          title: 'ZeroCentDNS - Free Educational DNS Platform', 
          description: 'ZeroCentDNS is a free educational platform that offers hands-on experience with DNS configuration via delegated subdomains. It provides a safe and accessible environment for individuals to understand core DNS concepts before investing in a top-level domain.',
          jsonLd: organizationJsonLd 
        };
    }
  };

  const seoData = getSeoData();
  const isDocsPage = location.pathname.startsWith(`/${locale}/docs`);

  return (
    <>
      <Seo {...seoData} />
      <Header setMobileMenuOpen={setIsMobileMenuOpen} />
      <MobileMenu isOpen={isMobileMenuOpen} onClose={() => setIsMobileMenuOpen(false)} />
      <main> {/* Menggunakan tag <main> untuk konten utama */}
        <Routes>
          <Route path="/" element={
            <>
              <Hero />
              <Features />
              <Statistics />
              <About />
              <Partners />
              <EmailServiceOffer />
            </>
          } />
          <Route path="/abuse-report" element={<AbusePage />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/docs/*" element={<DocPageWrapper />} />
          <Route path="/grants" element={<GrantsPage />} />
          <Route path="/*" element={<Navigate to={`/${locale}`} replace />} />
        </Routes>
      </main>
      {!isDocsPage && <Footer />}
    </>
  );
}

