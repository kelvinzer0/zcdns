import { ArrowRight, Heart, Target, Zap } from "lucide-react"

import { Badge } from "./ui/badge" // Adjusted import path
import { Button } from "./ui/button" // Adjusted import path
import EducationIllustration from '../assets/Education-amico.svg';
import { Link } from 'react-router-dom'; // Changed to react-router-dom Link
import { useTranslations } from '../lib/useTranslations'; // Adjusted import path for placeholder

export function About() {
  const t = useTranslations('About');
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="container mx-auto max-w-6xl">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div className="space-y-8">
            <div className="space-y-4">
              <Badge
  variant="secondary"
  className="bg-gradient-to-r from-blue-100 to-blue-200 text-green-700 rounded-full py-2"
>
                <Heart className="w-4 h-4 mr-2" />
                {t('non-profit-initiative')}
              </Badge>
              <h2 className="text-3xl sm:text-4xl font-bold text-gray-900">{t('our-mission-and-values')}</h2>
              <p className="text-lg text-gray-600 leading-relaxed" dangerouslySetInnerHTML={{ __html: t('mission-description') }}>
              </p>
            </div>

            <div className="space-y-6">
              <div className="flex items-start space-x-4">
                <div className="p-2 bg-green-100  mt-1 rounded-xl">
                  <Target className="h-5 w-5 text-green-600 " />
                </div>
                <div>
                  <h3 className="font-semibold text-gray-900 mb-2">{t('educational-first')}</h3>
                  <p className="text-gray-600">
                    {t('edufirst-description')}
                  </p>
                </div>
              </div>

              <div className="flex items-start space-x-4">
                <div className="p-2 bg-blue-100  mt-1 rounded-xl">
                  <Zap className="h-5 w-5 text-green-600 " />
                </div>
                <div>
                  <h3 className="font-semibold text-gray-900 mb-2">{t('efficiency-and-simplicity')}</h3>
                  <p className="text-gray-600">
                    {t('efficiency-description')}
                  </p>
                </div>
              </div>

              <div className="flex items-start space-x-4">
                <div className="p-2 bg-purple-100  mt-1 rounded-xl">
                  <Heart className="h-5 w-5 text-purple-600" />
                </div>
                <div>
                  <h3 className="font-semibold text-gray-900 mb-2">{t('community-driven')}</h3>
                  <p className="text-gray-600">
                    {t('community-driver-description')}
                  </p>
                </div>
              </div>
            </div>

            <Button
              asChild
              size="lg"
              className="bg-[#012241] hover:bg-[#02365f] text-white shadow-lg hover:shadow-xl transition-all duration-200"
            >
              <Link to="https://www.zcdns.id/dashboard" target="_blank">
                {t('start-your-dns-journey')}
                <ArrowRight className="ml-2 h-5 w-5" />
              </Link>
            </Button>
          </div>

          <div className="relative flex justify-center items-center">
            <img src={EducationIllustration} alt="Education Illustration" className="w-full max-w-md" />
          </div>

        </div>
      </div>
    </section>
  )
}
