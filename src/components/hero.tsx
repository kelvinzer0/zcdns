import { AlarmClock, ArrowRight, BookOpen, CircuitBoard, Zap } from "lucide-react"

import { Badge } from "./ui/badge"
import { Button } from "./ui/button"
import { CustomVideoSlider } from "./CustomVideoSlider";
import { LocalizedLink as Link } from './LocalizedLink';
import { useTranslations } from '../lib/useTranslations';

export function Hero() {
  const t = useTranslations('Hero');
  const videos = [
    {
      src: "https://codeberg.org/zcdns/mediakit/raw/branch/main/videos/zcdns-1.mp4",
      title: "Introduction to ZeroCentDNS",
      description: "Learn the basics of DNS and how ZeroCentDNS can help you.",
      poster: "https://codeberg.org/zcdns/mediakit/raw/branch/main/videos/zcdns-1.webp"
    },
    {
      src: "https://codeberg.org/zcdns/mediakit/raw/branch/main/videos/zcdns-2.webm",
      title: "Getting Started with ZeroCentDNS",
      description: "A step-by-step guide to setting up your first DNS record.",
      poster: "https://codeberg.org/zcdns/mediakit/raw/branch/main/videos/zcdns-2.png"
    }
  ];

  return (
    <section className="relative pt-32 pb-20 px-4 sm:px-6 lg:px-8 overflow-hidden">
      {/* Background Bubbles */}
      <div className="absolute top-0 left-0 w-64 h-64 bg-blue-100  opacity-50 filter blur-2xl -translate-x-1/4 -translate-y-1/4"></div>
      <div className="absolute bottom-0 right-0 w-72 h-72 bg-green-100  opacity-40 filter blur-3xl translate-x-1/4 translate-y-1/4"></div>
      
      <div className="container mx-auto max-w-6xl relative z-10">
        <div className="text-center space-y-8">
          {/* Badge */}
          <Badge variant="secondary" className="bg-blue-100 text-green-700 hover:bg-blue-200 px-4 py-2 rounded-full">
            <Zap className="w-4 h-4 mr-2" />
            {t('100-free-for-dns-education-purposes')}
          </Badge>

          {/* Main Heading */}
          <div className="space-y-4">
            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 leading-tight">
              {t('learn-dns-configuration')}
              <span className="block text-green-600">{t('risk-free-and-hands-on')}</span>
            </h1>
            <p className="text-xl text-gray-600 max-w-3xl mx-auto leading-relaxed">
              {t('risk-free-description')}
            </p>
          </div>

          {/* CTA Buttons */}
          <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
            <Button
              asChild
              size="lg"
              className="bg-[#012241] hover:bg-[#02365f] text-white shadow-lg hover:shadow-xl transition-all duration-200 px-8 py-3"
            >
              <Link to="/dashboard">
                {t('start-learning-now')}
                <ArrowRight className="ml-2 h-5 w-5" />
              </Link>
            </Button>
            <Button
              variant="outline"
              size="lg"
              className="border-gray-300 hover:border-blue-300 hover:bg-blue-50 px-8 py-3"
            >
              <Link to="docs/overview" className="flex items-center">
                <BookOpen className="mr-2 h-5 w-5" />
                {t('view-documentation')}
              </Link>
            </Button>
          </div>

          {/* Trust Indicators */}
          <div className="pt-8 flex flex-col sm:flex-row items-center justify-center gap-6 text-sm text-gray-500">
            <div className="flex items-center gap-2">
              <CircuitBoard className="w-4 h-4 text-green-500" />
              <span>{t('unique-name-zcdns-id')}</span>
            </div>
            <div className="hidden sm:block w-1 h-1 bg-gray-300 " />
            <div className="flex items-center gap-2">
              <AlarmClock className="w-4 h-4 text-green-500" />
              <span>{t('6-months-active-period')}</span>
            </div>
          </div>
        </div>

        {/* Visual Element */}
        <div className="mt-16 relative">
          <div className="bg-white  p-2 shadow-xl flex justify-center items-center">
            <CustomVideoSlider videos={videos} />
          </div>
        </div>
      </div>
    </section>
  )
}
