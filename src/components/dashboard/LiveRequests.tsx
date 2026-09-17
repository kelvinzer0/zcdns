import { useState } from 'react';
import { Radio, Trash2, Search, Eye, CheckCircle2, XCircle } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import type { DnsRequestLog } from './types';

interface Props {
  requests: DnsRequestLog[];
  wsStatus: 'connected' | 'disconnected' | 'connecting';
  onClearRequests: () => Promise<void>;
}

export const LiveRequests: React.FC<Props> = ({
  requests,
  wsStatus,
  onClearRequests,
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedType, setSelectedType] = useState<string>('ALL');
  const [inspectModalLog, setInspectModalLog] = useState<DnsRequestLog | null>(null);

  const filteredRequests = requests.filter((req) => {
    const matchesSearch =
      req.qname.toLowerCase().includes(searchTerm.toLowerCase()) ||
      req.client_ip.includes(searchTerm) ||
      req.qtype.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesType = selectedType === 'ALL' || req.qtype.toUpperCase() === selectedType;
    return matchesSearch && matchesType;
  });

  const formatTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        fractionalSecondDigits: 3,
      });
    } catch {
      return isoString;
    }
  };

  const getRcodeBadge = (rcode: string) => {
    if (rcode === 'NOERROR') {
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-green-100 text-green-800">
          <CheckCircle2 className="w-3 h-3 mr-1 text-green-600" />
          NOERROR
        </span>
      );
    }
    if (rcode === 'NXDOMAIN') {
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-amber-100 text-amber-800">
          <XCircle className="w-3 h-3 mr-1 text-amber-600" />
          NXDOMAIN
        </span>
      );
    }
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-red-100 text-red-800">
        {rcode}
      </span>
    );
  };

  return (
    <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden space-y-0">
      {/* Header */}
      <div className="p-6 border-b border-gray-200 flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex items-center space-x-2">
            <h2 className="text-lg font-bold text-gray-900">Live DNS Query Stream</h2>
            <span
              className={`inline-flex items-center space-x-1 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                wsStatus === 'connected'
                  ? 'bg-green-100 text-green-700'
                  : wsStatus === 'connecting'
                  ? 'bg-amber-100 text-amber-700'
                  : 'bg-red-100 text-red-700'
              }`}
            >
              <span
                className={`w-2 h-2 rounded-full mr-1 ${
                  wsStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-gray-400'
                }`}
              />
              {wsStatus === 'connected' ? 'Live Streaming' : wsStatus}
            </span>
          </div>
          <p className="text-xs text-gray-500 mt-0.5">
            Real-time feed of all DNS queries arriving at the nameserver for your subdomain
          </p>
        </div>

        <div className="flex items-center space-x-3">
          {requests.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={onClearRequests}
              className="text-gray-600 hover:text-red-600 hover:border-red-200"
            >
              <Trash2 className="w-3.5 h-3.5 mr-1.5" />
              Clear Log
            </Button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="p-4 bg-gray-50 border-b border-gray-200 flex flex-wrap items-center gap-3">
        <div className="relative flex-1 min-w-[200px]">
          <Search className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <Input
            type="text"
            placeholder="Filter by domain name, IP, or type..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9 h-9 text-xs bg-white"
          />
        </div>

        <div className="flex items-center space-x-1.5 overflow-x-auto">
          {['ALL', 'A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'ANY'].map((t) => (
            <button
              key={t}
              onClick={() => setSelectedType(t)}
              className={`px-2.5 py-1 text-xs font-semibold rounded-md transition-colors ${
                selectedType === t
                  ? 'bg-[#012241] text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-200 border border-gray-200'
              }`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* Requests Feed Table */}
      {filteredRequests.length === 0 ? (
        <div className="p-16 text-center">
          <div className="w-12 h-12 rounded-full bg-blue-50 text-blue-500 mx-auto flex items-center justify-center mb-3">
            <Radio className="w-6 h-6 animate-pulse" />
          </div>
          <h3 className="text-base font-semibold text-gray-900 mb-1">
            {requests.length === 0 ? 'Waiting for incoming DNS queries...' : 'No queries match your filter'}
          </h3>
          <p className="text-sm text-gray-500 max-w-md mx-auto">
            {requests.length === 0
              ? 'Run `dig` in your terminal or use the In-Browser DNS Tester below. As soon as a query hits the nameserver, it will instantly appear here.'
              : 'Try clearing the search filter to see other queries.'}
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto max-h-[500px]">
          <table className="w-full text-left text-sm">
            <thead className="bg-gray-100 text-xs font-semibold text-gray-500 uppercase tracking-wider sticky top-0 z-10">
              <tr>
                <th className="py-2.5 px-6">Timestamp</th>
                <th className="py-2.5 px-4">Type</th>
                <th className="py-2.5 px-6">Query Name</th>
                <th className="py-2.5 px-4">Client IP</th>
                <th className="py-2.5 px-4">Status</th>
                <th className="py-2.5 px-6">Answers</th>
                <th className="py-2.5 px-4 text-right">Inspect</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 font-mono text-xs">
              {filteredRequests.map((req) => (
                <tr
                  key={req.id}
                  className="hover:bg-blue-50/40 transition-colors cursor-pointer"
                  onClick={() => setInspectModalLog(req)}
                >
                  <td className="py-3 px-6 text-gray-500 whitespace-nowrap">{formatTime(req.created_at)}</td>
                  <td className="py-3 px-4">
                    <span className="font-bold text-blue-700 bg-blue-50 border border-blue-200 px-2 py-0.5 rounded">
                      {req.qtype}
                    </span>
                  </td>
                  <td className="py-3 px-6 text-gray-900 font-medium truncate max-w-xs">{req.qname}</td>
                  <td className="py-3 px-4 text-gray-600 whitespace-nowrap">{req.client_ip}</td>
                  <td className="py-3 px-4 whitespace-nowrap">{getRcodeBadge(req.rcode)}</td>
                  <td className="py-3 px-6 text-gray-700 truncate max-w-sm">
                    {req.answers && req.answers.length > 0 ? (
                      <span className="text-gray-800 font-medium">{req.answers.join(', ')}</span>
                    ) : (
                      <span className="text-gray-400 italic">No answers</span>
                    )}
                  </td>
                  <td className="py-3 px-4 text-right whitespace-nowrap">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setInspectModalLog(req);
                      }}
                      className="text-gray-400 hover:text-blue-600 p-1 rounded hover:bg-gray-100"
                      title="Inspect DNS packet"
                    >
                      <Eye className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Inspect Modal */}
      {inspectModalLog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
          <div className="bg-white rounded-2xl max-w-2xl w-full border border-gray-200 shadow-2xl overflow-hidden animate-in fade-in zoom-in-95">
            <div className="p-6 border-b border-gray-200 flex items-center justify-between">
              <div>
                <h3 className="text-lg font-bold text-gray-900 flex items-center space-x-2">
                  <span>DNS Query Inspector</span>
                  {getRcodeBadge(inspectModalLog.rcode)}
                </h3>
                <p className="text-xs text-gray-500 font-mono mt-0.5">
                  ID: {inspectModalLog.id}
                </p>
              </div>
              <button
                onClick={() => setInspectModalLog(null)}
                className="text-gray-400 hover:text-gray-600 p-2 rounded-lg hover:bg-gray-100"
              >
                ✕
              </button>
            </div>

            <div className="p-6 space-y-4 font-mono text-xs max-h-[70vh] overflow-y-auto">
              <div className="grid grid-cols-2 gap-4 bg-gray-50 p-4 rounded-xl border border-gray-200">
                <div>
                  <span className="text-gray-500 block">Query Name</span>
                  <span className="font-bold text-gray-900 break-all">{inspectModalLog.qname}</span>
                </div>
                <div>
                  <span className="text-gray-500 block">Query Type</span>
                  <span className="font-bold text-blue-700">{inspectModalLog.qtype}</span>
                </div>
                <div>
                  <span className="text-gray-500 block">Client / Resolver IP</span>
                  <span className="font-bold text-gray-800">{inspectModalLog.client_ip}</span>
                </div>
                <div>
                  <span className="text-gray-500 block">Timestamp</span>
                  <span className="font-bold text-gray-800">{formatTime(inspectModalLog.created_at)}</span>
                </div>
              </div>

              <div>
                <h4 className="font-semibold text-gray-900 mb-2 uppercase tracking-wide text-[11px]">
                  Response Answers ({inspectModalLog.answers ? inspectModalLog.answers.length : 0})
                </h4>
                {inspectModalLog.answers && inspectModalLog.answers.length > 0 ? (
                  <div className="bg-gray-900 text-green-400 p-3 rounded-lg overflow-x-auto space-y-1">
                    {inspectModalLog.answers.map((ans, idx) => (
                      <div key={idx}>{ans}</div>
                    ))}
                  </div>
                ) : (
                  <div className="bg-gray-100 text-gray-500 p-3 rounded-lg italic">
                    Empty Answer section (NODATA / NXDOMAIN)
                  </div>
                )}
              </div>
            </div>

            <div className="p-4 bg-gray-50 border-t border-gray-200 flex justify-end">
              <Button size="sm" onClick={() => setInspectModalLog(null)}>
                Close
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
