import { createContext, useContext, useMemo, useState, useEffect } from 'react';
import type { ReactNode } from 'react';
import { useParams } from 'react-router-dom';
import { loadMessages } from './messages'; // Changed to loadMessages
import { routing } from './routing';

interface I18nContextType {
  t: (key: string, replacements?: Record<string, string | number>) => string;
  locale: string;
  loading: boolean; // Add loading state
}

const I18nContext = createContext<I18nContextType | undefined>(undefined);

export function I18nProvider({ children, locale: propLocale }: { children: ReactNode, locale?: string }) {
  const { locale: routeLocale } = useParams<{ locale: string }>();
  const [currentMessages, setCurrentMessages] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const locale = useMemo(() => {
    const finalLocale = propLocale || routeLocale;
    return finalLocale && routing.locales.includes(finalLocale as any)
      ? finalLocale
      : routing.defaultLocale;
  }, [propLocale, routeLocale]);

  useEffect(() => {
    setLoading(true);
    loadMessages(locale as any)
      .then(messages => {
        setCurrentMessages(messages);
        setLoading(false);
      })
      .catch(error => {
        console.error("Failed to load messages:", error);
        setCurrentMessages({}); // Fallback to empty messages on error
        setLoading(false);
      });
  }, [locale]);

  const t = (key: string, replacements?: Record<string, string | number>): string => {
    if (loading || !currentMessages) return key; // Return key if still loading or messages not loaded
    try {
      const keys = key.split('.');
      let message: any = currentMessages;
      for (const k of keys) {
        if (message === undefined || message === null) return key; // Handle undefined/null intermediate keys
        message = message[k];
      }
      if (typeof message === 'string') {
        let interpolatedMessage = message;
        if (replacements) {
          for (const repKey in replacements) {
            interpolatedMessage = interpolatedMessage.replace(`{${repKey}}`, String(replacements[repKey]));
          }
        }
        // Handle custom highlight tags
        interpolatedMessage = interpolatedMessage.replace(/\{\{highlight\}\}(.*?)\{\{\/highlight\}\}/g, '<span class="highlight-text">$1</span>');
        return interpolatedMessage;
      }
      return key;
    } catch (error) {
      console.error(`Error translating key "${key}":`, error);
      return key;
    }
  };

  return (
    <I18nContext.Provider value={{ t, locale, loading }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useLocale() {
  const context = useContext(I18nContext);
  if (context === undefined) {
    throw new Error('useLocale must be used within an I18nProvider');
  }
  return context.locale;
}

export { I18nContext };


