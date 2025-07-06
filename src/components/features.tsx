import { Card, CardContent } from "./ui/card" // Adjusted import path
import { Clock, GraduationCap, Lightbulb, Settings, Shield, Users } from "lucide-react"
import { useTranslations } from '../lib/useTranslations'; // Adjusted import path for placeholder


export function Features() {
  const t = useTranslations('Features');
  const features = [
    {
      icon: GraduationCap,
      title: t('educational-focus'),
      description: t('edufocus-description'),
    },
    {
      icon: Users,
      title: t('for-everyone'),
      description: t('for-everyone-description'),
    },
    {
      icon: Settings,
      title: t('full-dns-control'),
      description: t('full-dns-control-description'),
    },
    {
      icon: Clock,
      title: t('6-month-lifecycle'),
      description: t('6month-description'),
    },
    {
      icon: Shield,
      title: t('risk-free-learning'),
      description: t('nocost-description'),
    },
    {
      icon: Lightbulb,
      title: t('hands-on-experience'),
      description: t('handson-description'),
    },
  ]
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-white">
      <div className="container mx-auto max-w-6xl">
        <div className="text-center space-y-4 mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900">{t('why-choose-zerocentdns')}</h2>
          <p className="text-xl text-gray-600 max-w-2xl mx-auto">
            {t('why-description')}
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {features.map((feature, index) => (
            <Card
              key={index}
              className="border-0 shadow-lg hover:shadow-xl transition-all duration-300 hover:-translate-y-1 bg-gradient-to-br from-white to-gray-50"
            >
              <CardContent className="p-6">
                <div className="flex items-center space-x-4 mb-4">
                  <div className="p-3 bg-blue-100 rounded-lg">
                    <feature.icon className="h-6 w-6 text-green-600" />
                  </div>
                  <h3 className="text-xl font-semibold text-gray-900">{feature.title}</h3>
                </div>
                <p className="text-gray-600 leading-relaxed">{feature.description}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}
