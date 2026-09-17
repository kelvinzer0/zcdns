import { useState } from 'react';
import { Sparkles, ArrowRight, Check, Copy, ChevronDown, ChevronUp } from 'lucide-react';
import { Button } from '../ui/button';
import type { Experiment } from './types';

interface Props {
  subdomain: string;
  baseDomain: string;
  dnsPort: number;
  onSelectExperiment: (exp: Experiment) => void;
}

const EXPERIMENTS: Experiment[] = [
  {
    id: 'a-record-basics',
    title: '1. Create and Resolve Your First A Record',
    difficulty: 'Beginner',
    description: 'Learn how IPv4 mapping works and how DNS clients query address records.',
    steps: [
      'Add an A record for "web" pointing to 192.0.2.42 with 60s TTL',
      'Query it using terminal dig or the in-browser tester',
      'Notice the NOERROR status and the 192.0.2.42 answer in your live requests log',
    ],
    suggestedRecord: {
      name: 'web',
      type: 'A',
      value: '192.0.2.42',
      ttl: 60,
    },
    suggestedQuery: {
      name: 'web',
      type: 'A',
    },
    explanation:
      'A (Address) records translate human-readable names into 32-bit IPv4 addresses. Every time you open a website, your browser queries an A or AAAA record behind the scenes.',
  },
  {
    id: 'cname-alias',
    title: '2. CNAME Aliasing and Canonical Names',
    difficulty: 'Beginner',
    description: 'Understand how CNAME records redirect queries to other canonical hostnames.',
    steps: [
      'Add a CNAME record for "docs" pointing to "readme.example.com."',
      'Query the "docs" subdomain for A or CNAME',
      'Observe how DNS servers return the canonical alias target',
    ],
    suggestedRecord: {
      name: 'docs',
      type: 'CNAME',
      value: 'readme.example.com',
      ttl: 60,
    },
    suggestedQuery: {
      name: 'docs',
      type: 'CNAME',
    },
    explanation:
      'A CNAME record creates an alias pointing one name to another. Important rule: you cannot have other records (like MX or TXT) alongside a CNAME with the exact same name.',
  },
  {
    id: 'nxdomain-caching',
    title: '3. Non-Existent Domains (NXDOMAIN)',
    difficulty: 'Intermediate',
    description: 'Explore what happens when you query a hostname that was never registered.',
    steps: [
      'Query a random hostname like "secret-unicorn" that has no record',
      'Observe the response code: NXDOMAIN (RCODE 3)',
      'Notice the SOA record in the Authority section indicating negative caching TTL',
    ],
    suggestedQuery: {
      name: 'secret-unicorn',
      type: 'A',
    },
    explanation:
      'When a name does not exist, authoritative DNS servers reply with NXDOMAIN and attach the Start of Authority (SOA) record so resolvers can cache the negative answer and avoid hammering the nameserver.',
  },
  {
    id: 'txt-verification',
    title: '4. Domain Verification & SPF using TXT Records',
    difficulty: 'Intermediate',
    description: 'See how search engines, email providers, and SSL authorities use text records.',
    steps: [
      'Add a TXT record for "@" with value "v=spf1 include:_spf.google.com ~all"',
      'Query TXT for your root subdomain',
      'Inspect the answer quotes and content',
    ],
    suggestedRecord: {
      name: '@',
      type: 'TXT',
      value: '"v=spf1 include:_spf.google.com ~all"',
      ttl: 60,
    },
    suggestedQuery: {
      name: '@',
      type: 'TXT',
    },
    explanation:
      'TXT records store human or machine-readable text. They are commonly used for Sender Policy Framework (SPF), DKIM public keys, and site ownership verification (e.g. Google Search Console).',
  },
  {
    id: 'mx-mail-priority',
    title: '5. Mail Routing (MX) with Priorities',
    difficulty: 'Advanced',
    description: 'Configure primary and backup mail servers and understand preference numbers.',
    steps: [
      'Add an MX record with priority 10: "10 mail1.example.com"',
      'Add a backup MX record with priority 20: "20 mail2.example.com"',
      'Query MX to see both mail servers listed in order of priority',
    ],
    suggestedRecord: {
      name: '@',
      type: 'MX',
      value: '10 mail1.example.com',
      ttl: 300,
    },
    suggestedQuery: {
      name: '@',
      type: 'MX',
    },
    explanation:
      'MX records tell sending mail servers where to deliver email for your domain. Senders always attempt the mail server with the lowest preference number first, falling back to higher numbers if unavailable.',
  },
];

export const Experiments: React.FC<Props> = ({
  subdomain,
  baseDomain,
  dnsPort,
  onSelectExperiment,
}) => {
  const [expandedId, setExpandedId] = useState<string | null>('a-record-basics');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const fullDomain = `${subdomain}.${baseDomain}`;

  const copyDig = (id: string, text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  return (
    <div className="bg-white border border-gray-200 p-4 sm:p-6 space-y-4 rounded-none">
      <div className="flex items-center space-x-3">
        <div className="w-8 h-8 bg-[#012241]/10 text-[#012241] flex items-center justify-center rounded-none font-bold">
          <Sparkles className="w-4 h-4" />
        </div>
        <div>
          <h2 className="text-base sm:text-lg font-bold text-gray-900">Eksperimen DNS</h2>
          <p className="text-xs text-gray-500">
            Skenario interaktif memahami cara kerja DNS record dan resolusi
          </p>
        </div>
      </div>

      <div className="space-y-3">
        {EXPERIMENTS.map((exp) => {
          const isExpanded = expandedId === exp.id;
          const queryTarget = exp.suggestedQuery
            ? exp.suggestedQuery.name === '@'
              ? fullDomain
              : `${exp.suggestedQuery.name}.${fullDomain}`
            : fullDomain;
          const nameserver = `ns1.${baseDomain || 'zcdns.id'}`;
          const portFlag = dnsPort && dnsPort !== 53 ? ` -p ${dnsPort}` : '';
          const digCmd = `dig @${nameserver}${portFlag} ${queryTarget} ${exp.suggestedQuery?.type || 'A'}`;

          return (
            <div
              key={exp.id}
              className={`border transition-all rounded-none ${
                isExpanded ? 'border-[#012241] bg-slate-50/40' : 'border-gray-200 hover:border-gray-300'
              }`}
            >
              <div
                onClick={() => setExpandedId(isExpanded ? null : exp.id)}
                className="p-3.5 flex items-center justify-between cursor-pointer select-none"
              >
                <div className="flex items-center space-x-2.5">
                  <span
                    className={`text-[10px] font-bold px-1.5 py-0.5 uppercase rounded-none border ${
                      exp.difficulty === 'Beginner'
                        ? 'bg-green-50 text-green-800 border-green-200'
                        : exp.difficulty === 'Intermediate'
                        ? 'bg-amber-50 text-amber-800 border-amber-200'
                        : 'bg-purple-50 text-purple-800 border-purple-200'
                    }`}
                  >
                    {exp.difficulty}
                  </span>
                  <h3 className="text-sm font-bold text-gray-900">{exp.title}</h3>
                </div>
                <div className="text-gray-400">
                  {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </div>
              </div>

              {isExpanded && (
                <div className="px-4 pb-4 pt-1 space-y-3 text-xs border-t border-gray-100">
                  <p className="text-gray-600 leading-relaxed">{exp.description}</p>

                  <div className="bg-white p-3 border border-gray-200 space-y-1.5 rounded-none">
                    <span className="font-bold text-gray-700 block uppercase tracking-wider text-[11px]">
                      Langkah Eksperimen:
                    </span>
                    <ol className="list-decimal list-inside space-y-1 text-gray-600 font-medium">
                      {exp.steps.map((step, sIdx) => (
                        <li key={sIdx}>{step}</li>
                      ))}
                    </ol>
                  </div>

                  <div className="p-3 bg-blue-50/50 border border-blue-200 text-blue-950 leading-relaxed rounded-none">
                    <span className="font-bold block mb-1">Penjelasan:</span>
                    {exp.explanation}
                  </div>

                  {/* Terminal snippet */}
                  <div className="flex items-center justify-between bg-gray-900 text-green-400 p-2.5 rounded-none font-mono text-xs overflow-x-auto">
                    <span className="truncate mr-2">{digCmd}</span>
                    <button
                      type="button"
                      onClick={() => copyDig(exp.id, digCmd)}
                      className="text-gray-400 hover:text-white flex items-center space-x-1 flex-shrink-0"
                    >
                      {copiedId === exp.id ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-green-400" />
                          <span>Tersalin</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Salin</span>
                        </>
                      )}
                    </button>
                  </div>

                  <div className="flex justify-end pt-1">
                    <Button
                      size="sm"
                      onClick={() => onSelectExperiment(exp)}
                      className="bg-[#012241] hover:bg-[#02365f] text-white rounded-none h-8 text-xs font-semibold"
                    >
                      Coba Eksperimen
                      <ArrowRight className="w-3.5 h-3.5 ml-1.5" />
                    </Button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
