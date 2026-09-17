import { Navigate, Route, Routes, useLocation, useParams } from 'react-router-dom';

import { About } from './about';
import { AbusePage } from './abuse-page';
import { AdminPage } from './admin/AdminPage';
import { DashboardPage } from './dashboard/DashboardPage';
import { DocPageWrapper } from './doc-page-wrapper';
import { Features } from './features';
import { Footer } from './footer';
import { Header } from './header';
import { Hero } from './hero';
import { MobileMenu } from './mobile-menu';
import { Partners } from './partners';
import { Seo } from './Seo';
import { Statistics } from './Statistics';
import { routing } from '../i18n/routing';
import { useState } from 'react';

const organizationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  'name': 'ZeroCentDNS',
  'url': 'https://www.zcdns.id',
  'logo': 'https://www.zcdns.id/zcdns-light-logo.svg',
  'contactPoint': {
    '@type': 'ContactPoint',
    'contactType': 'customer support',
    'email': 'support@zcdns.id',
  },
};

export function LocalizedLayout() {
  const { locale } = useParams<{ locale: string }>();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  if (!locale || !routing.locales.includes(locale as (typeof routing.locales)[number])) {
    return <Navigate to={`/${routing.defaultLocale}`} replace />;
  }

  const getSeoData = () => {
    const path = location.pathname.replace(`/${locale}`, '') || '/';
    switch (path) {
      case '/about':
        return { title: 'About Us - ZeroCentDNS', description: 'Learn more about the mission and team behind ZCDNS.' };
      case '/abuse-report':
        return { title: 'Abuse Report - ZeroCentDNS', description: 'Report abuse or malicious activity related to our DNS services.' };
      case '/admin':
        return { title: 'Admin Abuse Management - ZeroCentDNS', description: 'Review and manage reported domains, abuse complaints, and enforce DNS suspensions.' };
      case '/dashboard':
        return { title: 'DNS Playground & Dashboard - ZeroCentDNS', description: 'Interactive DNS sandbox with live query stream, parental control, and record manager.' };
      default:
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
      <main>
        <Routes>
          <Route path="/" element={
            <>
              <Hero />
              <Features />
              <Statistics />
              <About />
              <Partners />
            </>
          } />
          <Route path="/abuse-report" element={<AbusePage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/docs/*" element={<DocPageWrapper />} />
          <Route path="/*" element={<Navigate to={`/${locale}`} replace />} />
        </Routes>
      </main>
      {!isDocsPage && <Footer />}
    </>
  );
}
