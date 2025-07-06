import { Button } from './ui/button';
import EmailIllustration from '../assets/Creative writing-bro.svg';
import { Link } from 'react-router-dom';
import { useTranslations } from '../lib/useTranslations';

export function EmailServiceOffer() {
  const t = useTranslations('EmailOffer');

  return (
    <section className="bg-gradient-to-br from-blue-50 to-white text-gray-800 py-16 px-4 sm:px-6 lg:px-8">
      <div className="container mx-auto max-w-6xl">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-12 items-center">
          {/* Illustration Column */}
          <div className="flex justify-center">
            <img src={EmailIllustration} alt="Email Service Illustration" className="w-full max-w-sm" />
          </div>
          
          {/* Text Content Column */}
          <div className="text-center md:text-left">
            <h2 className="text-3xl sm:text-4xl font-bold mb-4 leading-tight text-gray-900">
              {t('title')}
            </h2>
            <p className="text-lg sm:text-xl mb-8 text-gray-600 leading-relaxed"
              dangerouslySetInnerHTML={{ __html: t('description') }}
            ></p>
            <Button
              asChild
              size="lg"
              className="bg-[#012241] hover:bg-[#02365f] text-white shadow-lg hover:shadow-xl transition-all duration-200 px-8 py-3 text-lg font-semibold"
            >
              <Link to="http://webmail.zcdns.id" target="_blank" rel="noopener noreferrer">
                {t('access-webmail')}
              </Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}
