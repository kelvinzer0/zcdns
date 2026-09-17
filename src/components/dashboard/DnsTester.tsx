import { useState } from 'react';
import { Send, Terminal, Clock, CheckCircle2, XCircle, AlertTriangle, Copy, Check } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import type { TestQueryResult } from './types';
import { dashboardApi } from './api';

interface Props {
  subdomain: string;
  baseDomain: string;
}

const QUERY_TYPES = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'PTR', 'CAA', 'SRV', 'SOA', 'ANY'];

export const DnsTester: React.FC<Props> = ({ subdomain, baseDomain }) => {
  const fullDomain = `${subdomain}.${baseDomain}`;
  const [queryName, setQueryName] = useState(fullDomain);
  const [queryType, setQueryType] = useState('A');
  const [isLoading, setIsLoading] = useState(false);
  const [result, setResult] = useState<TestQueryResult | null>(null);
  const [copiedRaw, setCopiedRaw] = useState(false);

  const handleQuery = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!queryName.trim()) return;

    setIsLoading(true);
    try {
      const res = await dashboardApi.testQuery(queryName.trim(), queryType);
      setResult(res);
    } catch (err: any) {
      setResult({
        status: 'ERROR',
        rcode: 'ERROR',
        question: `${queryType} ${queryName}`,
        answers: [],
        authority: [],
        response_time_ms: 0,
        server: `ns1.${baseDomain || 'zcdns.id'}`,
        raw: err.message || 'DNS Query execution failed',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const copyRaw = () => {
    if (!result) return;
    navigator.clipboard.writeText(result.raw);
    setCopiedRaw(true);
    setTimeout(() => setCopiedRaw(false), 2000);
  };

  return (
    <div className="bg-white border border-gray-200 shadow-xs p-4 sm:p-6 space-y-4">
      <div>
        <div className="flex items-center space-x-2">
          <div className="w-8 h-8 bg-blue-100 text-blue-700 flex items-center justify-center font-bold">
            <Terminal className="w-4 h-4" />
          </div>
          <h2 className="text-base sm:text-lg font-bold text-gray-900">Web Dig (DNS Resolver)</h2>
        </div>
      </div>

      {/* Query Bar */}
      <form onSubmit={handleQuery} className="flex flex-col sm:flex-row gap-2 sm:gap-3 items-stretch sm:items-center">
        <select
          value={queryType}
          onChange={(e) => setQueryType(e.target.value)}
          className="h-10 px-3 border border-gray-300 text-sm bg-white font-bold text-gray-800 focus:ring-2 focus:ring-green-500 rounded-none w-full sm:w-auto"
        >
          {QUERY_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>

        <div className="relative flex-1">
          <Input
            type="text"
            placeholder={fullDomain}
            value={queryName}
            onChange={(e) => setQueryName(e.target.value)}
            className="h-10 text-xs sm:text-sm font-mono rounded-none"
          />
        </div>

        <Button
          type="submit"
          disabled={isLoading}
          className="bg-[#012241] hover:bg-[#02365f] text-white px-5 font-medium h-10 rounded-none w-full sm:w-auto"
        >
          <Send className="w-4 h-4 mr-1.5" />
          {isLoading ? 'Menguji...' : 'Kirim Query'}
        </Button>
      </form>

      {/* Quick Suggestions */}
      <div className="flex flex-wrap items-center gap-1.5 text-xs text-gray-500">
        <span className="font-bold text-gray-700">Uji cepat:</span>
        <button
          type="button"
          onClick={() => {
            setQueryName(fullDomain);
            setQueryType('A');
          }}
          className="px-2 py-0.5 bg-gray-50 hover:bg-gray-100 border border-gray-300 font-mono text-gray-700 rounded-none"
        >
          Root A
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(fullDomain);
            setQueryType('AAAA');
          }}
          className="px-2 py-0.5 bg-gray-50 hover:bg-gray-100 border border-gray-300 font-mono text-gray-700 rounded-none"
        >
          Root AAAA
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(fullDomain);
            setQueryType('SOA');
          }}
          className="px-2 py-0.5 bg-gray-50 hover:bg-gray-100 border border-gray-300 font-mono text-gray-700 rounded-none"
        >
          SOA
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(`test.${fullDomain}`);
            setQueryType('A');
          }}
          className="px-2 py-0.5 bg-gray-50 hover:bg-gray-100 border border-gray-300 font-mono text-gray-700 rounded-none"
        >
          test.{fullDomain}
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(`nonexistent.${fullDomain}`);
            setQueryType('A');
          }}
          className="px-2 py-0.5 bg-gray-50 hover:bg-gray-100 border border-gray-300 font-mono text-gray-700 rounded-none"
        >
          NXDOMAIN
        </button>
      </div>

      {/* Result Display */}
      {result && (
        <div className="space-y-3 pt-3 border-t border-gray-200">
          {/* Status bar */}
          <div className="flex flex-wrap items-center justify-between gap-2 bg-gray-50 p-2.5 sm:p-3 border border-gray-200 text-xs">
            <div className="flex items-center space-x-2 font-mono">
              <span className="font-bold text-gray-700">Status:</span>
              {result.rcode === 'NOERROR' ? (
                <span className="inline-flex items-center text-green-700 font-bold bg-green-100 px-1.5 py-0.2 border border-green-200 rounded-none">
                  <CheckCircle2 className="w-3 h-3 mr-1" />
                  NOERROR
                </span>
              ) : result.rcode === 'NXDOMAIN' ? (
                <span className="inline-flex items-center text-amber-700 font-bold bg-amber-100 px-1.5 py-0.2 border border-amber-200 rounded-none">
                  <XCircle className="w-3 h-3 mr-1" />
                  NXDOMAIN
                </span>
              ) : (
                <span className="inline-flex items-center text-red-700 font-bold bg-red-100 px-1.5 py-0.2 border border-red-200 rounded-none">
                  <AlertTriangle className="w-3 h-3 mr-1" />
                  {result.rcode}
                </span>
              )}

              <span className="text-gray-400">|</span>
              <span className="text-gray-600">Server: {result.server}</span>
            </div>

            <div className="flex items-center space-x-1.5 text-gray-500 font-mono">
              <Clock className="w-3 h-3" />
              <span>{result.response_time_ms} ms</span>
            </div>
          </div>

          {/* Answers Visual Cards */}
          {result.answers && result.answers.length > 0 && (
            <div className="space-y-1.5">
              <h4 className="text-[11px] font-bold text-gray-700 uppercase tracking-wider font-mono">
                Jawaban ({result.answers.length})
              </h4>
              <div className="grid grid-cols-1 gap-1.5">
                {result.answers.map((ans, idx) => (
                  <div
                    key={idx}
                    className="p-2.5 bg-green-50/60 border border-green-200 text-green-950 font-mono text-xs flex items-center justify-between rounded-none break-all"
                  >
                    <span>{ans}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Authority Visual Cards */}
          {result.authority && result.authority.length > 0 && (
            <div className="space-y-1.5">
              <h4 className="text-[11px] font-bold text-gray-700 uppercase tracking-wider font-mono">
                Authority ({result.authority.length})
              </h4>
              <div className="grid grid-cols-1 gap-1.5">
                {result.authority.map((auth, idx) => (
                  <div
                    key={idx}
                    className="p-2.5 bg-blue-50/60 border border-blue-200 text-blue-950 font-mono text-xs rounded-none break-all"
                  >
                    {auth}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Raw Output Terminal Box */}
          <div className="border border-gray-800 rounded-none overflow-hidden">
            <div className="flex items-center justify-between bg-gray-800 text-gray-300 px-3 py-1.5 text-xs font-mono">
              <span>dig output</span>
              <button
                type="button"
                onClick={copyRaw}
                className="text-gray-400 hover:text-white flex items-center space-x-1"
              >
                {copiedRaw ? (
                  <>
                    <Check className="w-3 h-3 text-green-400" />
                    <span className="text-green-400">Tersalin</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3 h-3" />
                    <span>Salin</span>
                  </>
                )}
              </button>
            </div>
            <pre className="p-3 bg-gray-900 text-green-400 font-mono text-xs overflow-x-auto max-h-64 leading-relaxed whitespace-pre-wrap">
              {result.raw}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
