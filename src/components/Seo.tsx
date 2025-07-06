// src/components/Seo.tsx
import { Helmet } from 'react-helmet-async';
import { useLocation } from 'react-router-dom';

interface SeoProps {
  title?: string;
  description?: string;
  canonical?: string;
  ogImage?: string;
  jsonLd?: Record<string, unknown>;
}

const BASE_URL = 'https://www.zcdns.id';
const DEFAULT_TITLE = 'ZeroCentDNS - Free Educational DNS Platform';
const DEFAULT_DESCRIPTION = 'ZeroCentDNS is a free educational platform that offers hands-on experience with DNS configuration via delegated subdomains. It provides a safe and accessible environment for individuals to understand core DNS concepts before investing in a top-level domain.';
const DEFAULT_OG_IMAGE = `${BASE_URL}/zcdns-og-image.png`; // Anda perlu membuat gambar ini

export function Seo({ 
  title, 
  description, 
  canonical,
  ogImage,
  jsonLd 
}: SeoProps) {
  const { pathname } = useLocation();

  const seo = {
    title: title ? `${title} | ZCDNS` : DEFAULT_TITLE,
    description: description || DEFAULT_DESCRIPTION,
    canonical: canonical || `${BASE_URL}${pathname}`,
    ogImage: ogImage || DEFAULT_OG_IMAGE,
  };

  return (
    <Helmet>
      {/* Standard SEO Tags */}
      <title>{seo.title}</title>
      <meta name="description" content={seo.description} />
      <link rel="canonical" href={seo.canonical} />

      {/* Open Graph Tags for Social Media */}
      <meta property="og:type" content="website" />
      <meta property="og:url" content={seo.canonical} />
      <meta property="og:title" content={seo.title} />
      <meta property="og:description" content={seo.description} />
      <meta property="og:image" content={seo.ogImage} />
      <meta property="og:site_name" content="ZCDNS" />

      {/* Twitter Card Tags */}
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:title" content={seo.title} />
      <meta name="twitter:description" content={seo.description} />
      <meta name="twitter:image" content={seo.ogImage} />

      {/* JSON-LD Structured Data */}
      {jsonLd && (
        <script type="application/ld+json">
          {JSON.stringify(jsonLd)}
        </script>
      )}
    </Helmet>
  );
}
