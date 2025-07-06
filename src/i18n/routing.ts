export const locales = ['en', 'id'] as const
export type Locale = typeof locales[number]

export const routing = {
    locales,
    defaultLocale: 'en' as Locale
}
