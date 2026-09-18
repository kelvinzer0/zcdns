import { useState } from 'react';
import { Plus, Trash2, Edit2, AlertCircle, Check, Copy, Info } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import type { DnsRecord, RecordType } from './types';
import { useTranslations } from '../../lib/useTranslations';

interface Props {
  subdomain: string;
  baseDomain: string;
  records: DnsRecord[];
  isLoading: boolean;
  onAddRecord: (record: { name: string; type: RecordType; value: string; ttl: number }) => Promise<void>;
  onUpdateRecord: (id: string, record: { name: string; type: RecordType; value: string; ttl: number }) => Promise<void>;
  onDeleteRecord: (id: string) => Promise<void>;
  onClearAll: () => Promise<void>;
}

const RECORD_TYPES: RecordType[] = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'PTR', 'CAA', 'SRV'];

const TYPE_DESCRIPTIONS: Record<RecordType, { placeholder: string; hint: string }> = {
  A: {
    placeholder: '192.0.2.1',
    hint: 'IPv4 Address. E.g. points to your server IP.',
  },
  AAAA: {
    placeholder: '2001:db8::1',
    hint: 'IPv6 Address. E.g. modern 128-bit IP address.',
  },
  CNAME: {
    placeholder: 'target.example.com',
    hint: 'Canonical Name alias pointing to another domain.',
  },
  TXT: {
    placeholder: '"v=spf1 ~all" or "sample-text"',
    hint: 'Arbitrary text data, often used for SPF or verification tokens.',
  },
  MX: {
    placeholder: '10 mail.example.com',
    hint: 'Mail Exchange (Priority + Mail Server Host).',
  },
  NS: {
    placeholder: 'ns1.example.com',
    hint: 'Authoritative Name Server for a sub-delegation.',
  },
  PTR: {
    placeholder: 'host.example.com',
    hint: 'Pointer record for reverse DNS mapping.',
  },
  CAA: {
    placeholder: '0 issue "letsencrypt.org"',
    hint: 'Certificate Authority Authorization (flag tag value).',
  },
  SRV: {
    placeholder: '10 60 5060 sipserver.example.com',
    hint: 'Service locator: Priority Weight Port Target.',
  },
};

export const RecordManager: React.FC<Props> = ({
  subdomain,
  baseDomain,
  records,
  isLoading,
  onAddRecord,
  onUpdateRecord,
  onDeleteRecord,
  onClearAll,
}) => {
  const t = useTranslations('Dashboard');
  const [type, setType] = useState<RecordType>('A');
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [ttl, setTtl] = useState(60);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Edit state
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editType, setEditType] = useState<RecordType>('A');
  const [editName, setEditName] = useState('');
  const [editValue, setEditValue] = useState('');
  const [editTtl, setEditTtl] = useState(60);

  const [copiedId, setCopiedId] = useState<string | null>(null);

  const fullDomain = `${subdomain}.${baseDomain}`;
  const previewFqdn = name.trim() === '' || name.trim() === '@' 
    ? fullDomain 
    : `${name.trim().toLowerCase()}.${fullDomain}`;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    if (!value.trim()) {
      setErrorMessage('Record value is required');
      return;
    }

    try {
      setIsSubmitting(true);
      await onAddRecord({
        name: name.trim() || '@',
        type,
        value: value.trim(),
        ttl: Number(ttl) || 60,
      });
      setName('');
      setValue('');
    } catch (err: any) {
      setErrorMessage(err.message || 'Failed to add record');
    } finally {
      setIsSubmitting(false);
    }
  };

  const startEdit = (rec: DnsRecord) => {
    setEditingId(rec.id);
    setEditType(rec.type);
    setEditName(rec.name);
    setEditValue(rec.value);
    setEditTtl(rec.ttl);
  };

  const handleSaveEdit = async () => {
    if (!editingId) return;
    try {
      await onUpdateRecord(editingId, {
        name: editName.trim() || '@',
        type: editType,
        value: editValue.trim(),
        ttl: Number(editTtl) || 60,
      });
      setEditingId(null);
    } catch (err: any) {
      alert(err.message || 'Failed to update record');
    }
  };

  const copyRecordValue = (id: string, text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const getTypeBadgeClass = (t: RecordType) => {
    switch (t) {
      case 'A':
        return 'bg-blue-100 text-blue-800 border-blue-200';
      case 'AAAA':
        return 'bg-purple-100 text-purple-800 border-purple-200';
      case 'CNAME':
        return 'bg-amber-100 text-amber-800 border-amber-200';
      case 'TXT':
        return 'bg-emerald-100 text-emerald-800 border-emerald-200';
      case 'MX':
        return 'bg-cyan-100 text-cyan-800 border-cyan-200';
      case 'NS':
        return 'bg-rose-100 text-rose-800 border-rose-200';
      case 'CAA':
        return 'bg-indigo-100 text-indigo-800 border-indigo-200';
      default:
        return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  return (
    <div className="space-y-6">
      {/* Add Record Card */}
      <div className="bg-white border border-gray-200 shadow-xs p-4 sm:p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center space-x-2">
            <div className="w-8 h-8 bg-green-100 text-green-700 flex items-center justify-center font-bold">
              <Plus className="w-4 h-4" />
            </div>
            <h2 className="text-base sm:text-lg font-bold text-gray-900">{t('records-title')}</h2>
          </div>
        </div>

        {errorMessage && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 text-xs sm:text-sm flex items-center space-x-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{errorMessage}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-12 gap-3 sm:gap-4">
            {/* Type */}
            <div className="sm:col-span-2">
              <label className="block text-xs font-bold text-gray-700 mb-1 uppercase">{t('col-type')}</label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as RecordType)}
                className="w-full h-10 px-3 border border-gray-300 text-sm bg-white focus:ring-2 focus:ring-green-500 font-semibold text-gray-800 rounded-none"
              >
                {RECORD_TYPES.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </div>

            {/* Name / Subdomain */}
            <div className="sm:col-span-4">
              <label className="block text-xs font-bold text-gray-700 mb-1 uppercase">
                {t('col-name')} <span className="text-gray-400 font-normal">(@)</span>
              </label>
              <Input
                type="text"
                placeholder={t('name-placeholder')}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="h-10 text-sm font-mono rounded-none"
              />
              <div className="text-[11px] text-gray-500 mt-1 truncate">
                FQDN: <code className="text-green-700 font-semibold">{previewFqdn}</code>
              </div>
            </div>

            {/* Value */}
            <div className="sm:col-span-4">
              <label className="block text-xs font-bold text-gray-700 mb-1 uppercase">{t('col-value')}</label>
              <Input
                type="text"
                placeholder={TYPE_DESCRIPTIONS[type].placeholder}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                className="h-10 text-sm font-mono rounded-none"
              />
              <div className="text-[11px] text-gray-500 mt-1 truncate">
                {TYPE_DESCRIPTIONS[type].hint}
              </div>
            </div>

            {/* TTL */}
            <div className="sm:col-span-2">
              <label className="block text-xs font-bold text-gray-700 mb-1 uppercase">{t('col-ttl')}</label>
              <select
                value={ttl}
                onChange={(e) => setTtl(Number(e.target.value))}
                className="w-full h-10 px-3 border border-gray-300 text-sm bg-white focus:ring-2 focus:ring-green-500 font-mono rounded-none"
              >
                <option value={5}>5s</option>
                <option value={60}>60s</option>
                <option value={300}>300s</option>
                <option value={3600}>3600s</option>
                <option value={86400}>86400s</option>
              </select>
            </div>
          </div>

          <div className="flex justify-end pt-1">
            <Button
              type="submit"
              disabled={isSubmitting}
              className="bg-[#012241] hover:bg-[#02365f] text-white px-6 font-medium rounded-none h-10 w-full sm:w-auto"
            >
              <Plus className="w-4 h-4 mr-1.5" />
              {isSubmitting ? '...' : t('records-add-btn')}
            </Button>
          </div>
        </form>
      </div>
      {/* All DNS Records Table */}
      <div className="bg-white border border-gray-200 shadow-xs">
        <div className="p-4 sm:p-5 border-b border-gray-200 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center space-x-2">
            <h2 className="text-base sm:text-lg font-bold text-gray-900">{t('records-title')}</h2>
            <span className="text-xs bg-gray-100 text-gray-700 font-mono font-bold px-2 py-0.5 border border-gray-200">
              {records.length}
            </span>
          </div>

          {records.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={onClearAll}
              className="text-red-600 border-red-200 hover:bg-red-50 hover:border-red-300 rounded-none h-8 text-xs"
            >
              <Trash2 className="w-3.5 h-3.5 mr-1" />
              {t('btn-delete')}
            </Button>
          )}
        </div>

        {isLoading ? (
          <div className="p-0">
            <div className="animate-pulse flex flex-col">
              {/* Header skeleton */}
              <div className="bg-gray-50 border-b border-gray-200 py-3 px-5 flex gap-4">
                <div className="h-3 w-1/4 bg-gray-200 rounded"></div>
                <div className="h-3 w-16 bg-gray-200 rounded"></div>
                <div className="h-3 w-1/3 bg-gray-200 rounded"></div>
                <div className="h-3 w-12 bg-gray-200 rounded"></div>
                <div className="h-3 w-12 bg-gray-200 rounded ml-auto"></div>
              </div>
              {/* Rows skeleton */}
              {[1, 2, 3].map(i => (
                <div key={i} className="py-3.5 px-5 flex items-center gap-4 border-b border-gray-100">
                  <div className="h-4 w-1/4 bg-gray-200 rounded"></div>
                  <div className="h-5 w-12 bg-gray-200 rounded"></div>
                  <div className="h-4 w-1/3 bg-gray-200 rounded"></div>
                  <div className="h-4 w-10 bg-gray-200 rounded"></div>
                  <div className="flex gap-2 ml-auto">
                    <div className="h-6 w-6 bg-gray-200 rounded"></div>
                    <div className="h-6 w-6 bg-gray-200 rounded"></div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : records.length === 0 ? (
          <div className="p-8 sm:p-12 text-center">
            <div className="w-10 h-10 bg-gray-100 text-gray-400 mx-auto flex items-center justify-center mb-2">
              <Info className="w-5 h-5" />
            </div>
            <h3 className="text-sm font-bold text-gray-800 mb-1">{t('records-empty')}</h3>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs sm:text-sm">
              <thead className="bg-gray-50 border-b border-gray-200 text-[11px] font-bold text-gray-600 uppercase tracking-wider font-mono">
                <tr>
                  <th className="py-2.5 px-3 sm:px-5">{t('col-name')}</th>
                  <th className="py-2.5 px-2 sm:px-3">{t('col-type')}</th>
                  <th className="py-2.5 px-3 sm:px-5">{t('col-value')}</th>
                  <th className="py-2.5 px-2 sm:px-3">{t('col-ttl')}</th>
                  <th className="py-2.5 px-3 sm:px-5 text-right">{t('col-actions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 font-mono">
                {records.map((rec) => {
                  const isEditing = editingId === rec.id;

                  if (isEditing) {
                    return (
                      <tr key={rec.id} className="bg-green-50/60">
                        <td className="py-2 px-3 sm:px-5">
                          <Input
                            type="text"
                            value={editName}
                            onChange={(e) => setEditName(e.target.value)}
                            className="h-8 text-xs font-mono rounded-none"
                          />
                        </td>
                        <td className="py-2 px-2 sm:px-3">
                          <select
                            value={editType}
                            onChange={(e) => setEditType(e.target.value as RecordType)}
                            className="h-8 px-1.5 border border-gray-300 text-xs bg-white font-semibold rounded-none"
                          >
                            {RECORD_TYPES.map((t) => (
                              <option key={t} value={t}>
                                {t}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td className="py-2 px-3 sm:px-5">
                          <Input
                            type="text"
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            className="h-8 text-xs font-mono rounded-none"
                          />
                        </td>
                        <td className="py-2 px-2 sm:px-3">
                          <input
                            type="number"
                            value={editTtl}
                            onChange={(e) => setEditTtl(Number(e.target.value))}
                            className="h-8 w-16 px-1.5 border border-gray-300 text-xs font-mono bg-white rounded-none"
                          />
                        </td>
                        <td className="py-2 px-3 sm:px-5 text-right space-x-1.5 whitespace-nowrap">
                          <Button size="sm" onClick={handleSaveEdit} className="bg-green-600 hover:bg-green-700 text-white h-7 px-2 text-xs rounded-none">
                            Simpan
                          </Button>
                          <Button size="sm" variant="ghost" onClick={() => setEditingId(null)} className="h-7 px-2 text-xs rounded-none">
                            Batal
                          </Button>
                        </td>
                      </tr>
                    );
                  }

                  return (
                    <tr key={rec.id} className="hover:bg-gray-50/70 transition-colors">
                      <td className="py-2.5 px-3 sm:px-5 font-mono text-gray-900">
                        <span className="text-green-700 font-bold">{rec.name === '@' ? '@' : rec.name}</span>
                        <span className="text-gray-400 font-normal">.{fullDomain}</span>
                      </td>
                      <td className="py-2.5 px-2 sm:px-3">
                        <span
                          className={`inline-block px-2 py-0.5 text-[11px] font-bold border rounded-none ${getTypeBadgeClass(
                            rec.type
                          )}`}
                        >
                          {rec.type}
                        </span>
                      </td>
                      <td className="py-2.5 px-3 sm:px-5 font-mono text-gray-800 break-all">
                        <div className="flex items-center space-x-1.5">
                          <span>{rec.value}</span>
                          <button
                            onClick={() => copyRecordValue(rec.id, rec.value)}
                            className="text-gray-400 hover:text-gray-600 transition-colors"
                            title="Salin nilai"
                          >
                            {copiedId === rec.id ? (
                              <Check className="w-3.5 h-3.5 text-green-600" />
                            ) : (
                              <Copy className="w-3.5 h-3.5" />
                            )}
                          </button>
                        </div>
                      </td>
                      <td className="py-2.5 px-2 sm:px-3 font-mono text-gray-500 text-xs">{rec.ttl}s</td>
                      <td className="py-2.5 px-3 sm:px-5 text-right space-x-1 whitespace-nowrap">
                        <button
                          onClick={() => startEdit(rec)}
                          className="p-1 text-gray-500 hover:text-blue-600 hover:bg-blue-50 transition-colors"
                          title="Edit"
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => onDeleteRecord(rec.id)}
                          className="p-1 text-gray-500 hover:text-red-600 hover:bg-red-50 transition-colors"
                          title="Hapus"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};
