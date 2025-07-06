import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  Title,
  Tooltip,
} from 'chart.js';
// src/components/GrantsPage.tsx
import { useEffect, useState } from 'react';

import { Bar } from 'react-chartjs-2';
import BrainstormingIllustration from '../assets/Brainstorming-cuate.svg';
import GrantsThumbnail from '../assets/oscar-mackey-fpuzyrkUtKE-unsplash.jpg';
import { useTranslations } from '../lib/useTranslations';
import { Seo } from './Seo';

ChartJS.register(
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend
);

interface GrantData {
  financial_overview: {
    year: number;
    monthly_data: { month: string; income: number; expenses: number; foss_donation: number }[];
    total_expenses: number;
    total_income: number;
    net_income: number;
    total_foss_donations: number;
  };
  foss_donations_breakdown: { project: string; amount: number }[];
  annual_reports: { year: number; url: string }[];
}

export function GrantsPage() {
  const t = useTranslations('GrantsPage');
  const [data, setData] = useState<GrantData | null>(null);

  useEffect(() => {
    fetch('/mock-api/grants.json')
      .then((res) => res.json())
      .then(setData)
      .catch(console.error);
  }, []);

  const chartOptions = {
    responsive: true,
    plugins: {
      legend: { position: 'top' as const },
      title: { display: true, text: t('monthly-overview-chart-title') },
    },
  };

  const chartData = {
    labels: data?.financial_overview.monthly_data.map(d => d.month) || [],
    datasets: [
      {
        label: t('income'),
        data: data?.financial_overview.monthly_data.map(d => d.income) || [],
        backgroundColor: 'rgba(75, 192, 192, 0.5)',
      },
      {
        label: t('expenses'),
        data: data?.financial_overview.monthly_data.map(d => d.expenses) || [],
        backgroundColor: 'rgba(255, 99, 132, 0.5)',
      },
      {
        label: t('foss-donations'),
        data: data?.financial_overview.monthly_data.map(d => d.foss_donation) || [],
        backgroundColor: 'rgba(54, 162, 235, 0.5)',
      },
    ],
  };

  return (
    <div className="bg-white">
      <Seo
        title="Grants & Donations - ZeroCentDNS"
        description="Support the ZeroCentDNS project through donations. This page provides details on our financial transparency, how to contribute, and how your support helps us maintain our free educational DNS services."
      />
      {/* Header Section */}
      <header className="text-center py-24 px-4 sm:px-6 lg:px-8 bg-gray-50">
        <h1 className="text-4xl font-extrabold text-gray-900 sm:text-5xl">{t('title')}</h1>
        <p className="mt-4 max-w-2xl mx-auto text-xl text-gray-500">{t('subtitle')}</p>
      </header>

      {/* Thumbnail Image */}
      <div className="w-full h-64 overflow-hidden">
        <img src={GrantsThumbnail} alt="Grants Page Thumbnail" className="w-full h-full object-cover" />
      </div>

      <main className="container mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {/* Why We Need Donations */}
        <section className="mb-16">
          <h2 className="text-3xl font-bold text-center mb-2">{t('no-free-service-title')}</h2>
          <p className="text-lg text-center text-gray-600 mb-8">{t('no-free-service-reason')}</p>
        </section>

        {/* Grid Layout */}
        <div className="grid lg:grid-cols-2 gap-16 items-start">
          {/* Left Column: Donation Methods & Benefits */}
          <div className="space-y-12">
            {/* Benefits */}
            <section>
              <h3 className="text-2xl font-bold mb-4">{t('benefits-title')}</h3>
              <ul className="list-disc list-inside space-y-2 text-gray-700">
                <li>{t('benefit-sustainability')}</li>
                <li>{t('benefit-free-dns')}</li>
                <li>{t('benefit-free-email')}</li>
              </ul>
            </section>

            {/* Monetary Donations */}
            <section>
              <h3 className="text-2xl font-bold mb-4">{t('monetary-title')}</h3>
              <div className="space-y-4">
                <p>{t('monetary-description')}</p>
                <div className="mt-6 space-y-6">
                  <div>
                    <p className="text-gray-700">Untuk donasi, Anda bisa menghubungi kami melalui email:</p>
                    <p className="font-mono bg-gray-100 p-3 mt-2">funding@zcdns.id</p>
                    <p className="text-red-600 mt-4">
                      <strong className="font-semibold">Peringatan Penipuan:</strong> Kami tidak pernah memberikan nomor rekening bank, PayPal, atau alamat dompet kripto selain melalui email resmi kami di funding@zcdns.id. Harap berhati-hati terhadap upaya penipuan.
                    </p>
                  </div>
                </div>
              </div>
            </section>

            {/* Hardware Donations */}
            <section>
              <h3 className="text-2xl font-bold mb-4">{t('hardware-title')}</h3>
              <p className="mb-4">{t('hardware-description')}</p>
              <p className="text-sm p-4 bg-gray-100 ">{t('hardware-examples')}</p>
              <p className="mt-4">{t('hardware-shipping')}</p>
            </section>
             <section>
                <h3 className="text-2xl font-bold mb-4">{t('contact-title')}</h3>
                <p>{t('contact-description')}</p>
                <div className="mt-4 space-y-2">
                    <p>{t('email-contact')}: <a href="mailto:funding@zcdns.id" className="text-blue-600 hover:underline">funding@zcdns.id</a></p>
                </div>
            </section>
          </div>

          {/* Right Column: Illustration & Transparency */}
          <div className="space-y-12">
            <img src={BrainstormingIllustration} alt={t('illustration-alt')} className="w-full" />
            
            {/* Financial Transparency */}
            <section>
              <h3 className="text-2xl font-bold mb-4">{t('transparency-title')}</h3>
              {data ? (
                <div className="space-y-8">
                  <div className="overflow-y-auto max-h-96">
                    <Bar options={chartOptions} data={chartData} />
                  </div>
                  {/* Financial Table */}
                  <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-gray-200">
                      <thead className="bg-gray-50">
                        <tr>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">{t('description')}</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">{t('amount')}</th>
                        </tr>
                      </thead>
                      <tbody className="bg-white divide-y divide-gray-200">
                        <tr><td className="px-6 py-4">Total Income</td><td className="px-6 py-4">${data.financial_overview.total_income}</td></tr>
                        <tr><td className="px-6 py-4">Total Expenses</td><td className="px-6 py-4">${data.financial_overview.total_expenses}</td></tr>
                        <tr><td className="px-6 py-4">Net Income ({data.financial_overview.year})</td><td className="px-6 py-4">${data.financial_overview.net_income}</td></tr>
                        <tr><td className="px-6 py-4">20% Donated to FOSS</td><td className="px-6 py-4">${data.financial_overview.total_foss_donations}</td></tr>
                      </tbody>
                    </table>
                  </div>
                  {/* FOSS Donations Breakdown */}
                  <h4 className="text-xl font-semibold mt-6">{t('foss-donations-title')}</h4>
                  <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-gray-200">
                      <thead className="bg-gray-50">
                        <tr>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">{t('project')}</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">{t('amount')} (USD)</th>
                        </tr>
                      </thead>
                      <tbody className="bg-white divide-y divide-gray-200">
                        {data.foss_donations_breakdown.map(d => (
                          <tr key={d.project}>
                            <td className="px-6 py-4 whitespace-nowrap">{d.project}</td>
                            <td className="px-6 py-4 whitespace-nowrap">${d.amount}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : <p>{t('loading-data')}</p>}
            </section>

            {/* Annual Reports */}
            <section>
              <h3 className="text-2xl font-bold mb-4">{t('reports-title')}</h3>
              <div className="space-y-3">
                {data?.annual_reports.map(report => (
                  <a key={report.year} href={report.url} target="_blank" rel="noopener noreferrer" className="flex items-center p-4 border  hover:bg-gray-50">
                    <p className="font-medium">{t('report-for-year', { year: report.year })}</p>
                  </a>
                ))}
              </div>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}