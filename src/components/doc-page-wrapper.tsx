import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { DocPageClient } from './doc-page-client';
import { Seo } from './Seo'; // Import the Seo component

interface SidebarItem {
    label: string
    slug?: string
    icon: string
    children?: {
        label: string
        slug: string
        icon: string
    }[]
}

interface DocIndexItem {
  locale: string;
  slug: string;
  // Add other properties from your docs-index.json if they exist
}

interface DocData {
  content: string;
  navigation: {
    previous: { label: string; slug: string } | null;
    next: { label: string; slug: string } | null;
  };
  sidebarItems: SidebarItem[];
  frontMatter: { // Add frontMatter to DocData interface
    title?: string;
    description?: string;
  };
}

export function DocPageWrapper() {
  const { locale, '*': slugParam } = useParams();
  const navigate = useNavigate();
  const slug = useMemo(() => {
    const parts = (slugParam || '').split('/').filter(p => p);
    return parts.length > 0 ? parts : ['overview'];
  }, [slugParam]);

  const [docData, setDocData] = useState<DocData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchDoc = async () => {
      setLoading(true);
      setError(null);
      try {
        // Fetch docs-index.json for sidebar and navigation
        const indexResponse = await fetch('/docs-index.json');
        if (!indexResponse.ok) throw new Error('Failed to fetch docs index');
        const docsIndex: DocIndexItem[] = await indexResponse.json();

        // Build sidebar items
        const sidebarItems = [
          { label: 'Overview', slug: 'overview', icon: 'BookOpen' },
          { label: 'Concepts', slug: 'concepts', icon: 'BookOpen' },
          { label: 'Get Started', slug: 'get-started', icon: 'BookOpen' },
          { label: 'Glossary', slug: 'glossary', icon: 'BookOpen' },
          {
            label: 'Records', icon: 'Folder', children: docsIndex
              .filter((item) => item.locale === locale && item.slug.startsWith('records/'))
              .map((item) => ({
                label: item.slug.split('/').pop()?.toUpperCase() ?? '',
                slug: item.slug,
                icon: 'FileText'
              }))
          },
        ];

        // Determine navigation
        const currentDocIndex = docsIndex.findIndex((item) => item.locale === locale && item.slug === slug.join('/'));
        const previousDoc = currentDocIndex > 0 ? docsIndex[currentDocIndex - 1] : null;
        const nextDoc = currentDocIndex < docsIndex.length - 1 ? docsIndex[currentDocIndex + 1] : null;

        const navigation = {
          previous: previousDoc ? { label: previousDoc.slug.split('/').pop() ?? '', slug: previousDoc.slug } : null,
          next: nextDoc ? { label: nextDoc.slug.split('/').pop() ?? '', slug: nextDoc.slug } : null,
        };

        // Fetch actual MDX content
        const contentPath = `/docs/${locale}/${slug.join('/')}.json`;
        const contentResponse = await fetch(contentPath);
        if (!contentResponse.ok) throw new Error(`Failed to fetch content for ${contentPath}`);
        const contentJson = await contentResponse.json();

        setDocData({
          content: contentJson.content,
          navigation,
          sidebarItems,
          frontMatter: contentJson.frontMatter, // Store frontMatter
        });
      } catch (err) {
        if (err instanceof Error) {
          setError(err.message);
        } else {
          setError('An unknown error occurred');
        }
      } finally {
        setLoading(false);
      }
    };

    if (locale && slug.length > 0) {
      fetchDoc();
    } else if (locale && slug.length === 0) {
      // Redirect to overview if no slug is provided
      navigate(`/${locale}/docs/overview`, { replace: true });
    }
  }, [locale, slug, navigate]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <svg className="animate-spin h-10 w-10 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>
    );
  }

  if (error) {
    return <div>Error: {error}</div>;
  }

  if (!docData) {
    return <div>Document not found.</div>;
  }

  return (
    <>
      <Seo title={docData.frontMatter.title} description={docData.frontMatter.description} />
      <DocPageClient
        locale={locale || 'en'}
        slug={slug}
        content={docData.content}
        navigation={docData.navigation}
        sidebarItems={docData.sidebarItems}
      />
    </>
  );
}
