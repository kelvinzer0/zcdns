import { Link as RouterLink } from 'react-router-dom';
import type { ComponentProps } from 'react';
import { useLocale } from '../i18n/I18nProvider';

export function LocalizedLink({ to, ...props }: ComponentProps<typeof RouterLink>) {
  const locale = useLocale();
  const localizedTo = to.toString().startsWith('/') ? `/${locale}${to}` : to;

  return <RouterLink to={localizedTo} {...props} />;
}
