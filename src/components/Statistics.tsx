// src/components/Statistics.tsx
import { useEffect, useState } from 'react';

import BusinessGrowthIllustration from '../assets/Business growth-cuate.svg';
import { useTranslations } from '../lib/useTranslations';

interface StatsData {
  users: number;
  activeSubdomains: number;
  ensUsed: number;
  infrastructureDonors: number;
}

export function Statistics() {
  const t = useTranslations('Statistics');
  const [stats, setStats] = useState<StatsData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/mock-api/stats.json')
      .then((res) => res.json())
      .then((data) => {
        setStats(data);
      })
      .catch((error) => console.error('Failed to fetch stats:', error))
      .finally(() => setLoading(false));
  }, []);

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      notation: 'compact',
      compactDisplay: 'short',
    }).format(num);
  };

  const statItems = stats ? [
    { label: t('users'), value: formatNumber(stats.users) },
    { label: t('active-subdomains'), value: formatNumber(stats.activeSubdomains) },
    { label: t('ens-used'), value: formatNumber(stats.ensUsed) },
    { label: t('infrastructure-donors'), value: formatNumber(stats.infrastructureDonors) },
  ] : [];

  return (
    <section className="bg-gray-50 py-20 sm:py-24">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8">
        <div className="grid md:grid-cols-2 gap-12 items-center">
          {/* Kolom Ilustrasi */}
          <div className="text-center md:text-left">
            <img
              src={BusinessGrowthIllustration}
              alt="Illustration of business growth and statistics"
              className="w-full max-w-md mx-auto"
            />
          </div>
          {/* Kolom Statistik */}
          <div className="space-y-8">
            <div>
              <h2 className="text-3xl font-bold text-gray-900 sm:text-4xl">
                {t('title')}
              </h2>
              <p className="mt-4 text-lg text-gray-600">
                {t('subtitle')}
              </p>
            </div>
            {loading ? (
              <p className="text-gray-500">{t('loading')}...</p>
            ) : stats ? (
              <div className="grid grid-cols-2 max-w-2xl mx-auto">
                {statItems.map((item, index) => {
                  // Tentukan kelas border berdasarkan posisi
                  let borderClasses = "";

                  if (index === 0) {
                    // Top-left: hanya border kanan dan bawah
                    borderClasses = "border-r border-b border-gray-200";
                  } else if (index === 1) {
                    // Top-right: hanya border bawah
                    borderClasses = "border-b border-gray-200";
                  } else if (index === 2) {
                    // Bottom-left: hanya border kanan
                    borderClasses = "border-r border-gray-200";
                  } else if (index === 3) {
                    // Bottom-right: tidak ada border internal
                    borderClasses = "";
                  }

                  return (
                    <div key={item.label} className={`p-6 ${borderClasses}`}>
                      <p className="text-4xl font-bold text-[#012241]">
                        {item.value}
                      </p>
                      <p className="mt-1 text-base text-gray-500">{item.label}</p>
                    </div>
                  );
                })}
              </div>
            ) : (
              <p className="text-red-500">{t('error')}</p>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
