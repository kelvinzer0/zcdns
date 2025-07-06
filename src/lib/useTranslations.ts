import { useContext } from 'react';
import { I18nContext } from '../i18n/I18nProvider';

export function useTranslations(namespace: string) {
  const context = useContext(I18nContext);
  if (context === undefined) {
    throw new Error('useTranslations must be used within an I18nProvider');
  }

  const { t } = context;

  return (key: string, replacements?: Record<string, string | number>) => t(`${namespace}.${key}`, replacements);
}
