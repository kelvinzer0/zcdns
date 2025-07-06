import { useTranslations } from '../lib/useTranslations';

const partners = [
  {
    name: 'Pandi',
    logoUrl: '/brands/pandi-logo.png',
    website: 'https://pandi.id',
  },
  {
    name: '6project',
    logoUrl: '/brands/6project-logo.png',
    website: 'https://6project.org',
  },
];

export function Partners() {
  const t = useTranslations('Partners');

  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8">
      <div className="container mx-auto max-w-6xl text-center">
        <h2 className="text-3xl font-bold text-gray-800 mb-4">{t('in-collaboration-with')}</h2>
        <p className="text-lg text-gray-600 mb-12">{t('proudly-supported-by')}</p>
        <div className="flex flex-wrap justify-center items-center gap-8 sm:gap-12">
          {partners.map((partner) => (
            <a
              key={partner.name}
              href={partner.website}
              target="_blank"
              rel="noopener noreferrer"
              className="grayscale hover:grayscale-0 transition-all duration-300"
            >
              <img
                src={partner.logoUrl}
                alt={`${partner.name} Logo`}
                className="h-12 sm:h-16 object-contain"
              />
            </a>
          ))}
        </div>
      </div>
    </section>
  );
}
