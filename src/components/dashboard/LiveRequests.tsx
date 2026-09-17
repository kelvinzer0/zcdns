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
        <span className="inline-flex items-center px-1.5 py-0.2 text-[11px] font-bold bg-green-100 text-green-800 border border-green-200 rounded-none">
          <CheckCircle2 className="w-3 h-3 mr-1 text-green-600" />
          NOERROR
        </span>
      );
    }
    if (rcode === 'NXDOMAIN') {
      return (
        <span className="inline-flex items-center px-1.5 py-0.2 text-[11px] font-bold bg-amber-100 text-amber-800 border border-amber-200 rounded-none">
          <XCircle className="w-3 h-3 mr-1 text-amber-600" />
          NXDOMAIN
        </span>
      );
    }
    return (
      <span className="inline-flex items-center px-1.5 py-0.2 text-[11px] font-bold bg-red-100 text-red-800 border border-red-200 rounded-none">
        {rcode}
      </span>
    );
  };

  return (
    <div className="bg-white border border-gray-200 shadow-xs space-y-0">
      {/* Header */}
      <div className="p-4 sm:p-5 border-b border-gray-200 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center space-x-2">
          <h2 className="text-base sm:text-lg font-bold text-gray-900">Live DNS Stream</h2>
          <span
            className={`inline-flex items-center px-2 py-0.5 text-xs font-bold font-mono border rounded-none ${
              wsStatus === 'connected'
                ? 'bg-green-50 text-green-700 border-green-200'
                : wsStatus === 'connecting'
                ? 'bg-amber-50 text-amber-700 border-amber-200'
                : 'bg-red-50 text-red-700 border-red-200'
            }`}
          >
            <span
              className={`w-1.5 h-1.5 mr-1.5 ${
                wsStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-gray-400'
              }`}
            />
            {wsStatus === 'connected' ? 'LIVE' : wsStatus.toUpperCase()}
          </span>
        </div>

        <div className="flex items-center space-x-2">
          {requests.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={onClearRequests}
              className="text-gray-600 hover:text-red-600 hover:border-red-200 rounded-none h-8 text-xs"
            >
              <Trash2 className="w-3.5 h-3.5 mr-1" />
              Bersihkan
            </Button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="p-3 sm:p-4 bg-gray-50 border-b border-gray-200 flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3">
        <div className="relative flex-1">
          <Search className="w-3.5 h-3.5 text-gray-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <Input
            type="text"
            placeholder="Cari domain, IP, atau tipe..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-8 h-8 text-xs bg-white rounded-none"
          />
        </div>

        <div className="flex items-center gap-1 overflow-x-auto pb-1 sm:pb-0 scrollbar-none">
          {['ALL', 'A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'ANY'].map((t) => (
            <button
              key={t}
              onClick={() => setSelectedType(t)}
              className={`px-2 py-0.5 text-xs font-mono font-semibold transition-colors rounded-none ${
                selectedType === t
                  ? 'bg-[#012241] text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-200 border border-gray-300'
              }`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* Requests Feed Table */}
      {filteredRequests.length === 0 ? (
        <div className="p-8 sm:p-12 text-center">
          <div className="w-10 h-10 bg-blue-50 text-blue-600 mx-auto flex items-center justify-center mb-2">
            <Radio className="w-5 h-5 animate-pulse" />
          </div>
          <h3 className="text-sm font-bold text-gray-900 mb-1">
            {requests.length === 0 ? 'Menunggu kueri DNS masuk...' : 'Tidak ada kueri yang cocok'}
          </h3>
          <p className="text-xs text-gray-500 max-w-sm mx-auto">
            {requests.length === 0
              ? 'Jalankan kueri dig atau uji via Web Dig untuk melihat aliran paket DNS masuk secara live.'
              : 'Coba ubah kata kunci atau reset filter tipe kueri.'}
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto max-h-[500px]">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-gray-100 text-[11px] font-bold text-gray-600 uppercase tracking-wider sticky top-0 z-10 border-b border-gray-200">
              <tr>
                <th className="py-2 px-3 sm:px-4">Waktu</th>
                <th className="py-2 px-2 sm:px-3">Tipe</th>
                <th className="py-2 px-3 sm:px-4">Query Name</th>
                <th className="py-2 px-2 sm:px-3">Client IP</th>
                <th className="py-2 px-2 sm:px-3">Status</th>
                <th className="py-2 px-3 sm:px-4">Jawaban</th>
                <th className="py-2 px-2 sm:px-3 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredRequests.map((req) => (
                <tr
                  key={req.id}
                  className="hover:bg-blue-50/40 transition-colors cursor-pointer"
                  onClick={() => setInspectModalLog(req)}
                >
                  <td className="py-2.5 px-3 sm:px-4 text-gray-500 whitespace-nowrap">{formatTime(req.created_at)}</td>
                  <td className="py-2.5 px-2 sm:px-3">
                    <span className="font-bold text-blue-700 bg-blue-50 border border-blue-200 px-1.5 py-0.2 rounded-none">
                      {req.qtype}
                    </span>
                  </td>
                  <td className="py-2.5 px-3 sm:px-4 text-gray-900 font-bold truncate max-w-[140px] sm:max-w-xs">{req.qname}</td>
                  <td className="py-2.5 px-2 sm:px-3 text-gray-600 whitespace-nowrap">{req.client_ip}</td>
                  <td className="py-2.5 px-2 sm:px-3 whitespace-nowrap">{getRcodeBadge(req.rcode)}</td>
                  <td className="py-2.5 px-3 sm:px-4 text-gray-700 truncate max-w-[150px] sm:max-w-sm">
                    {req.answers && req.answers.length > 0 ? (
                      <span className="text-gray-800 font-medium">{req.answers.join(', ')}</span>
                    ) : (
                      <span className="text-gray-400 italic">Tanpa jawaban</span>
                    )}
                  </td>
                  <td className="py-2.5 px-2 sm:px-3 text-right whitespace-nowrap">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setInspectModalLog(req);
                      }}
                      className="text-gray-400 hover:text-blue-600 p-1 hover:bg-gray-100 rounded-none"
                      title="Detail paket"
                    >
                      <Eye className="w-3.5 h-3.5" />
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-3 sm:p-4 backdrop-blur-xs">
          <div className="bg-white max-w-xl w-full border border-gray-300 shadow-2xl overflow-hidden rounded-none animate-in fade-in zoom-in-95">
            <div className="p-4 sm:p-5 border-b border-gray-200 flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <h3 className="text-base font-bold text-gray-900">Inspeksi Paket DNS</h3>
                {getRcodeBadge(inspectModalLog.rcode)}
              </div>
              <button
                onClick={() => setInspectModalLog(null)}
                className="text-gray-400 hover:text-gray-600 p-1.5 hover:bg-gray-100 rounded-none text-sm font-bold"
              >
                ✕
              </button>
            </div>

            <div className="p-4 sm:p-5 space-y-3 font-mono text-xs max-h-[70vh] overflow-y-auto">
              <div className="grid grid-cols-2 gap-3 bg-gray-50 p-3 border border-gray-200 rounded-none">
                <div>
                  <span className="text-gray-500 block text-[10px] uppercase">Query Name</span>
                  <span className="font-bold text-gray-900 break-all">{inspectModalLog.qname}</span>
                </div>
                <div>
                  <span className="text-gray-500 block text-[10px] uppercase">Query Type</span>
                  <span className="font-bold text-blue-700">{inspectModalLog.qtype}</span>
                </div>
                <div>
                  <span className="text-gray-500 block text-[10px] uppercase">Client IP</span>
                  <span className="font-bold text-gray-800">{inspectModalLog.client_ip}</span>
                </div>
                <div>
                  <span className="text-gray-500 block text-[10px] uppercase">Waktu</span>
                  <span className="font-bold text-gray-800">{formatTime(inspectModalLog.created_at)}</span>
                </div>
              </div>

              <div>
                <h4 className="font-bold text-gray-800 mb-1.5 uppercase text-[11px]">
                  Jawaban DNS ({inspectModalLog.answers ? inspectModalLog.answers.length : 0})
                </h4>
                {inspectModalLog.answers && inspectModalLog.answers.length > 0 ? (
                  <div className="bg-gray-900 text-green-400 p-3 overflow-x-auto space-y-1 rounded-none">
                    {inspectModalLog.answers.map((ans, idx) => (
                      <div key={idx}>{ans}</div>
                    ))}
                  </div>
                ) : (
                  <div className="bg-gray-100 text-gray-500 p-2.5 italic rounded-none">
                    Bagian jawaban kosong (NODATA / NXDOMAIN)
                  </div>
                )}
              </div>
            </div>

            <div className="p-3 sm:p-4 bg-gray-50 border-t border-gray-200 flex justify-end">
              <Button size="sm" onClick={() => setInspectModalLog(null)} className="rounded-none h-8 px-4 text-xs">
                Tutup
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
