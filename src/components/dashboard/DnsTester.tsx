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
    <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-6">
      <div>
        <div className="flex items-center space-x-2">
          <div className="w-8 h-8 rounded-lg bg-blue-100 text-blue-700 flex items-center justify-center font-bold">
            <Terminal className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-gray-900">In-Browser DNS Resolver ("Web Dig")</h2>
            <p className="text-xs text-gray-500">
              Test queries instantly against your nameserver without leaving the browser
            </p>
          </div>
        </div>
      </div>

      {/* Query Bar */}
      <form onSubmit={handleQuery} className="flex flex-col sm:flex-row gap-3 items-stretch sm:items-center">
        <select
          value={queryType}
          onChange={(e) => setQueryType(e.target.value)}
          className="h-10 px-3 border border-gray-300 rounded-lg text-sm bg-white font-bold text-gray-800 focus:ring-2 focus:ring-green-500"
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
            className="h-10 text-sm font-mono"
          />
        </div>

        <Button
          type="submit"
          disabled={isLoading}
          className="bg-[#012241] hover:bg-[#02365f] text-white px-6 font-medium h-10"
        >
          <Send className="w-4 h-4 mr-2" />
          {isLoading ? 'Querying...' : 'Resolve Query'}
        </Button>
      </form>

      {/* Quick Suggestions */}
      <div className="flex flex-wrap items-center gap-2 text-xs text-gray-500">
        <span className="font-medium text-gray-700">Quick tests:</span>
        <button
          type="button"
          onClick={() => {
            setQueryName(fullDomain);
            setQueryType('A');
          }}
          className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 rounded-md font-mono text-gray-700"
        >
          Root A record
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(`test.${fullDomain}`);
            setQueryType('A');
          }}
          className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 rounded-md font-mono text-gray-700"
        >
          test.{fullDomain}
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(fullDomain);
            setQueryType('TXT');
          }}
          className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 rounded-md font-mono text-gray-700"
        >
          TXT record
        </button>
        <button
          type="button"
          onClick={() => {
            setQueryName(`nonexistent.${fullDomain}`);
            setQueryType('A');
          }}
          className="px-2.5 py-1 bg-gray-100 hover:bg-gray-200 rounded-md font-mono text-gray-700"
        >
          NXDOMAIN test
        </button>
      </div>

      {/* Result Display */}
      {result && (
        <div className="space-y-4 pt-4 border-t border-gray-100">
          {/* Status bar */}
          <div className="flex flex-wrap items-center justify-between gap-3 bg-gray-50 p-3.5 rounded-xl border border-gray-200 text-xs">
            <div className="flex items-center space-x-3">
              <span className="font-semibold text-gray-700">Status:</span>
              {result.rcode === 'NOERROR' ? (
                <span className="inline-flex items-center text-green-700 font-bold bg-green-100 px-2 py-0.5 rounded">
                  <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
                  NOERROR
                </span>
              ) : result.rcode === 'NXDOMAIN' ? (
                <span className="inline-flex items-center text-amber-700 font-bold bg-amber-100 px-2 py-0.5 rounded">
                  <XCircle className="w-3.5 h-3.5 mr-1" />
                  NXDOMAIN (Non-Existent Domain)
                </span>
              ) : (
                <span className="inline-flex items-center text-red-700 font-bold bg-red-100 px-2 py-0.5 rounded">
                  <AlertTriangle className="w-3.5 h-3.5 mr-1" />
                  {result.rcode}
                </span>
              )}

              <span className="text-gray-400">|</span>
              <span className="text-gray-600 font-mono">Server: {result.server}</span>
            </div>

            <div className="flex items-center space-x-2 text-gray-500 font-mono">
              <Clock className="w-3.5 h-3.5" />
              <span>{result.response_time_ms} ms</span>
            </div>
          </div>

          {/* Answers Visual Cards */}
          {result.answers && result.answers.length > 0 && (
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wider">
                Answers ({result.answers.length})
              </h4>
              <div className="grid grid-cols-1 gap-2">
                {result.answers.map((ans, idx) => (
                  <div
                    key={idx}
                    className="p-3 rounded-lg bg-green-50/60 border border-green-200 text-green-950 font-mono text-xs flex items-center justify-between"
                  >
                    <span>{ans}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Authority Visual Cards */}
          {result.authority && result.authority.length > 0 && (
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wider">
                Authority Section ({result.authority.length})
              </h4>
              <div className="grid grid-cols-1 gap-2">
                {result.authority.map((auth, idx) => (
                  <div
                    key={idx}
                    className="p-3 rounded-lg bg-blue-50/60 border border-blue-200 text-blue-950 font-mono text-xs"
                  >
                    {auth}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Raw Output Terminal Box */}
          <div className="relative">
            <div className="flex items-center justify-between bg-gray-800 text-gray-300 px-4 py-2 rounded-t-xl text-xs font-mono">
              <span>dig output</span>
              <button
                type="button"
                onClick={copyRaw}
                className="text-gray-400 hover:text-white flex items-center space-x-1"
              >
                {copiedRaw ? (
                  <>
                    <Check className="w-3.5 h-3.5 text-green-400" />
                    <span className="text-green-400">Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3.5 h-3.5" />
                    <span>Copy raw</span>
                  </>
                )}
              </button>
            </div>
            <pre className="p-4 bg-gray-900 text-green-400 font-mono text-xs rounded-b-xl overflow-x-auto max-h-72 leading-relaxed whitespace-pre-wrap">
              {result.raw}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
