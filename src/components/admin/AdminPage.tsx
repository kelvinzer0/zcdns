// src/components/admin/AdminPage.tsx
import { useState, useEffect } from "react";
import {
  ShieldAlert,
  ShieldCheck,
  Search,
  Lock,
  LogOut,
  CheckCircle2,
  Trash2,
  Ban,
  Clock,
  Activity,
  FileText,
  RefreshCw,
  Eye,
  X,
  Server,
} from "lucide-react";
import { Seo } from "../Seo";
import { useTranslations } from "../../lib/useTranslations";

interface AbuseReport {
  id: string;
  reporter_name: string;
  reporter_email: string;
  abuse_type: string;
  subdomain: string;
  description: string;
  evidence: string;
  status: "pending" | "investigating" | "resolved_blocked" | "dismissed";
  admin_notes: string;
  created_at: string;
  updated_at: string;
}

interface BlockedSubdomain {
  subdomain: string;
  reason: string;
  blocked_at: string;
}

interface GrowthStats {
  active_subdomains: number;
  active_records: number;
  total_queries: number;
  blocked_threats: number;
  blocked_domains: number;
}

export function AdminPage() {
  const t = useTranslations("Admin");

  const [adminToken, setAdminToken] = useState<string | null>(() => {
    return localStorage.getItem("zcdns_admin_token");
  });

  const [loginPassword, setLoginPassword] = useState("");
  const [loginError, setLoginError] = useState<string | null>(null);
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  // Dashboard state
  const [activeTab, setActiveTab] = useState<"reports" | "blocked" | "stats">("reports");
  const [reports, setReports] = useState<AbuseReport[]>([]);
  const [blockedList, setBlockedList] = useState<BlockedSubdomain[]>([]);
  const [growthStats, setGrowthStats] = useState<GrowthStats | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  // Filters
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [searchQuery, setSearchQuery] = useState("");

  // Modal
  const [selectedReport, setSelectedReport] = useState<AbuseReport | null>(null);
  const [modalAdminNotes, setModalAdminNotes] = useState("");

  // Manual block form
  const [manualSubdomain, setManualSubdomain] = useState("");
  const [manualReason, setManualReason] = useState("");
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  const fetchAdminData = async (token = adminToken) => {
    if (!token) return;
    setIsLoading(true);
    try {
      const headers = { "X-Admin-Key": token };

      const [reportsRes, blockedRes, statsRes] = await Promise.all([
        fetch("/api/admin/abuse-reports", { headers }),
        fetch("/api/admin/blocked-subdomains", { headers }),
        fetch("/api/admin/stats", { headers }),
      ]);

      if (reportsRes.status === 401 || blockedRes.status === 401) {
        handleLogout();
        return;
      }

      if (reportsRes.ok) {
        const data = await reportsRes.json();
        setReports(data);
      }

      if (blockedRes.ok) {
        const data = await blockedRes.json();
        setBlockedList(data);
      }

      if (statsRes.ok) {
        const data = await statsRes.json();
        setGrowthStats(data.growth);
      }
    } catch (err) {
      console.error("Error fetching admin data:", err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (adminToken) {
      fetchAdminData(adminToken);
    }
  }, [adminToken]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginError(null);
    setIsLoggingIn(true);

    try {
      const res = await fetch("/api/admin/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: loginPassword }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Password admin salah");
      }

      const token = data.token || loginPassword;
      localStorage.setItem("zcdns_admin_token", token);
      setAdminToken(token);
      setLoginPassword("");
      fetchAdminData(token);
    } catch (err: any) {
      setLoginError(err.message || "Gagal masuk");
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem("zcdns_admin_token");
    setAdminToken(null);
    setReports([]);
    setBlockedList([]);
    setGrowthStats(null);
  };

  const handleUpdateStatus = async (
    id: string,
    status: AbuseReport["status"],
    blockSubdomain = false,
    notes?: string
  ) => {
    if (!adminToken) return;
    try {
      const res = await fetch(`/api/admin/abuse-reports/${id}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "X-Admin-Key": adminToken,
        },
        body: JSON.stringify({
          status,
          admin_notes: notes !== undefined ? notes : (selectedReport?.admin_notes || ""),
          block_subdomain: blockSubdomain,
        }),
      });

      if (res.ok) {
        setActionSuccess(
          blockSubdomain
            ? `Subdomain berhasil diblokir dan laporan ditandai selesai.`
            : `Status laporan berhasil diperbarui menjadi ${status}.`
        );
        fetchAdminData();
        if (selectedReport && selectedReport.id === id) {
          setSelectedReport(null);
        }
      }
    } catch (err) {
      console.error("Failed to update status:", err);
    }
  };

  const handleDeleteReport = async (id: string) => {
    if (!adminToken) return;
    if (!confirm("Apakah Anda yakin ingin menghapus laporan ini?")) return;

    try {
      const res = await fetch(`/api/admin/abuse-reports/${id}`, {
        method: "DELETE",
        headers: { "X-Admin-Key": adminToken },
      });

      if (res.ok) {
        setActionSuccess("Laporan berhasil dihapus.");
        fetchAdminData();
      }
    } catch (err) {
      console.error("Failed to delete report:", err);
    }
  };

  const handleManualBlock = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!adminToken || !manualSubdomain) return;

    try {
      const res = await fetch("/api/admin/blocked-subdomains", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Admin-Key": adminToken,
        },
        body: JSON.stringify({
          subdomain: manualSubdomain,
          reason: manualReason || "Blocked by administrator",
        }),
      });

      if (res.ok) {
        setActionSuccess(`Subdomain ${manualSubdomain} berhasil diblokir.`);
        setManualSubdomain("");
        setManualReason("");
        fetchAdminData();
      }
    } catch (err) {
      console.error("Failed to block subdomain:", err);
    }
  };

  const handleUnblock = async (subdomain: string) => {
    if (!adminToken) return;
    if (!confirm(`Buka blokir untuk subdomain ${subdomain}?`)) return;

    try {
      const res = await fetch(`/api/admin/blocked-subdomains/${subdomain}`, {
        method: "DELETE",
        headers: { "X-Admin-Key": adminToken },
      });

      if (res.ok) {
        setActionSuccess(`Subdomain ${subdomain} berhasil dibuka blokirnya.`);
        fetchAdminData();
      }
    } catch (err) {
      console.error("Failed to unblock subdomain:", err);
    }
  };

  const filteredReports = reports.filter((rep) => {
    const matchesStatus =
      statusFilter === "all" ? true : rep.status === statusFilter;
    const matchesSearch =
      searchQuery === ""
        ? true
        : rep.subdomain.toLowerCase().includes(searchQuery.toLowerCase()) ||
          rep.reporter_email.toLowerCase().includes(searchQuery.toLowerCase()) ||
          rep.reporter_name.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesStatus && matchesSearch;
  });

  const pendingCount = reports.filter((r) => r.status === "pending").length;
  const investigatingCount = reports.filter((r) => r.status === "investigating").length;

  if (!adminToken) {
    return (
      <main className="pt-28 pb-20 px-4 sm:px-6 lg:px-8 min-h-screen bg-gray-50 flex items-center justify-center">
        <Seo
          title="Admin Login - ZeroCentDNS"
          description="Administrator portal for ZeroCentDNS abuse management."
        />
        <div className="max-w-md w-full bg-white rounded-2xl shadow-sm border border-gray-200 p-8">
          <div className="text-center mb-6">
            <div className="w-12 h-12 bg-red-100 rounded-2xl flex items-center justify-center mx-auto mb-3">
              <Lock className="w-6 h-6 text-red-600" />
            </div>
            <h1 className="text-2xl font-bold text-gray-900">{t("login-title")}</h1>
            <p className="text-sm text-gray-500 mt-1">{t("login-desc")}</p>
          </div>

          {loginError && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-xl text-xs text-red-700">
              {loginError}
            </div>
          )}

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Admin Password
              </label>
              <input
                type="password"
                required
                value={loginPassword}
                onChange={(e) => setLoginPassword(e.target.value)}
                placeholder={t("password-placeholder")}
                className="w-full px-4 py-2.5 rounded-lg border border-gray-200 focus:outline-none focus:ring-2 focus:ring-red-500 text-sm"
              />
            </div>
            <button
              type="submit"
              disabled={isLoggingIn}
              className="w-full py-2.5 bg-[#012241] hover:bg-[#02365f] text-white font-medium rounded-lg text-sm transition-all flex items-center justify-center gap-2 shadow-sm disabled:opacity-50"
            >
              {isLoggingIn ? "Memproses..." : t("login-button")}
            </button>
          </form>
        </div>
      </main>
    );
  }

  return (
    <main className="pt-24 pb-20 px-4 sm:px-6 lg:px-8 min-h-screen bg-gray-50">
      <Seo
        title="Admin Abuse Management - ZeroCentDNS"
        description="Portal Administrator ZeroCentDNS untuk memantau dan memblokir penyalahgunaan domain."
      />
      <div className="container mx-auto max-w-7xl">
        {/* Top Header */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-white p-6 rounded-2xl border border-gray-200 shadow-sm mb-8">
          <div>
            <div className="flex items-center gap-3">
              <div className="p-2 bg-red-100 rounded-xl">
                <ShieldAlert className="w-6 h-6 text-red-600" />
              </div>
              <div>
                <h1 className="text-2xl font-extrabold text-gray-900">{t("title")}</h1>
                <p className="text-sm text-gray-500">{t("subtitle")}</p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => fetchAdminData()}
              disabled={isLoading}
              className="p-2.5 bg-gray-100 hover:bg-gray-200 rounded-xl text-gray-700 transition-colors"
              title="Refresh Data"
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? "animate-spin" : ""}`} />
            </button>
            <button
              onClick={handleLogout}
              className="inline-flex items-center gap-2 px-4 py-2.5 bg-red-50 hover:bg-red-100 text-red-700 rounded-xl text-sm font-medium transition-colors border border-red-200"
            >
              <LogOut className="w-4 h-4" />
              {t("logout")}
            </button>
          </div>
        </div>

        {/* Success Alert */}
        {actionSuccess && (
          <div className="mb-6 p-4 bg-green-50 border border-green-200 rounded-xl flex items-center justify-between text-green-800 text-sm">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="w-5 h-5 text-green-600" />
              <span>{actionSuccess}</span>
            </div>
            <button onClick={() => setActionSuccess(null)} className="text-green-600 hover:text-green-800">
              <X className="w-4 h-4" />
            </button>
          </div>
        )}

        {/* Metric Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm">
            <div className="flex items-center justify-between text-gray-500 text-xs font-semibold uppercase mb-2">
              <span>Total Laporan</span>
              <FileText className="w-4 h-4 text-gray-400" />
            </div>
            <p className="text-3xl font-extrabold text-gray-900">{reports.length}</p>
          </div>

          <div className="bg-white p-5 rounded-2xl border border-yellow-200 shadow-sm bg-yellow-50/20">
            <div className="flex items-center justify-between text-yellow-700 text-xs font-semibold uppercase mb-2">
              <span>Menunggu Ditinjau</span>
              <Clock className="w-4 h-4 text-yellow-600" />
            </div>
            <p className="text-3xl font-extrabold text-yellow-800">{pendingCount}</p>
          </div>

          <div className="bg-white p-5 rounded-2xl border border-blue-200 shadow-sm bg-blue-50/20">
            <div className="flex items-center justify-between text-blue-700 text-xs font-semibold uppercase mb-2">
              <span>Sedang Diinvestigasi</span>
              <Activity className="w-4 h-4 text-blue-600" />
            </div>
            <p className="text-3xl font-extrabold text-blue-800">{investigatingCount}</p>
          </div>

          <div className="bg-white p-5 rounded-2xl border border-red-200 shadow-sm bg-red-50/20">
            <div className="flex items-center justify-between text-red-700 text-xs font-semibold uppercase mb-2">
              <span>Subdomain Diblokir</span>
              <Ban className="w-4 h-4 text-red-600" />
            </div>
            <p className="text-3xl font-extrabold text-red-800">{blockedList.length}</p>
          </div>
        </div>

        {/* Navigation Tabs */}
        <div className="flex gap-2 border-b border-gray-200 mb-6">
          <button
            onClick={() => setActiveTab("reports")}
            className={`pb-3 px-4 text-sm font-semibold border-b-2 transition-all flex items-center gap-2 ${
              activeTab === "reports"
                ? "border-red-600 text-red-600"
                : "border-transparent text-gray-500 hover:text-gray-900"
            }`}
          >
            <ShieldAlert className="w-4 h-4" />
            {t("tab-reports")} ({reports.length})
          </button>
          <button
            onClick={() => setActiveTab("blocked")}
            className={`pb-3 px-4 text-sm font-semibold border-b-2 transition-all flex items-center gap-2 ${
              activeTab === "blocked"
                ? "border-red-600 text-red-600"
                : "border-transparent text-gray-500 hover:text-gray-900"
            }`}
          >
            <Ban className="w-4 h-4" />
            {t("tab-blocked")} ({blockedList.length})
          </button>
          <button
            onClick={() => setActiveTab("stats")}
            className={`pb-3 px-4 text-sm font-semibold border-b-2 transition-all flex items-center gap-2 ${
              activeTab === "stats"
                ? "border-red-600 text-red-600"
                : "border-transparent text-gray-500 hover:text-gray-900"
            }`}
          >
            <Server className="w-4 h-4" />
            {t("tab-stats")}
          </button>
        </div>

        {/* Tab 1: Abuse Reports */}
        {activeTab === "reports" && (
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            {/* Filter Bar */}
            <div className="p-4 border-b border-gray-100 flex flex-col md:flex-row gap-4 justify-between items-center bg-gray-50/50">
              <div className="flex flex-wrap items-center gap-2">
                {["all", "pending", "investigating", "resolved_blocked", "dismissed"].map((st) => (
                  <button
                    key={st}
                    onClick={() => setStatusFilter(st)}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold capitalize transition-colors ${
                      statusFilter === st
                        ? "bg-[#012241] text-white"
                        : "bg-white text-gray-600 border border-gray-200 hover:bg-gray-100"
                    }`}
                  >
                    {st === "all" ? t("status-all") : st.replace("_", " ")}
                  </button>
                ))}
              </div>

              <div className="relative w-full md:w-72">
                <Search className="w-4 h-4 absolute left-3 top-3 text-gray-400" />
                <input
                  type="text"
                  placeholder="Cari subdomain atau pelapor..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full pl-9 pr-4 py-2 bg-white rounded-xl border border-gray-200 text-xs focus:outline-none focus:ring-2 focus:ring-red-500"
                />
              </div>
            </div>

            {/* Table */}
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold border-b border-gray-100">
                  <tr>
                    <th className="py-3.5 px-4">{t("col-date")}</th>
                    <th className="py-3.5 px-4">{t("col-subdomain")}</th>
                    <th className="py-3.5 px-4">{t("col-type")}</th>
                    <th className="py-3.5 px-4">{t("col-reporter")}</th>
                    <th className="py-3.5 px-4">{t("col-status")}</th>
                    <th className="py-3.5 px-4 text-right">{t("col-action")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredReports.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="py-12 text-center text-gray-500">
                        {t("empty-reports")}
                      </td>
                    </tr>
                  ) : (
                    filteredReports.map((rep) => {
                      const isBlocked = blockedList.some((b) => b.subdomain === rep.subdomain);

                      let badgeClass = "bg-gray-100 text-gray-700";
                      if (rep.status === "pending") badgeClass = "bg-yellow-100 text-yellow-800 border-yellow-200";
                      if (rep.status === "investigating") badgeClass = "bg-blue-100 text-blue-800 border-blue-200";
                      if (rep.status === "resolved_blocked") badgeClass = "bg-red-100 text-red-800 border-red-200";
                      if (rep.status === "dismissed") badgeClass = "bg-gray-100 text-gray-600";

                      return (
                        <tr key={rep.id} className="hover:bg-gray-50/80 transition-colors">
                          <td className="py-3.5 px-4 text-xs text-gray-500 whitespace-nowrap">
                            {new Date(rep.created_at).toLocaleDateString("id-ID", {
                              month: "short",
                              day: "numeric",
                              hour: "2-digit",
                              minute: "2-digit",
                            })}
                          </td>
                          <td className="py-3.5 px-4 font-mono font-bold text-gray-900">
                            {rep.subdomain}.zcdns.id
                            {isBlocked && (
                              <span className="ml-2 px-1.5 py-0.5 bg-red-100 text-red-700 text-[10px] rounded font-sans uppercase">
                                Terblokir
                              </span>
                            )}
                          </td>
                          <td className="py-3.5 px-4">
                            <span className="px-2 py-0.5 bg-gray-100 text-gray-700 rounded text-xs capitalize font-medium">
                              {rep.abuse_type}
                            </span>
                          </td>
                          <td className="py-3.5 px-4 text-xs">
                            <div className="font-semibold text-gray-900">{rep.reporter_name || "Anonim"}</div>
                            <div className="text-gray-500">{rep.reporter_email}</div>
                          </td>
                          <td className="py-3.5 px-4">
                            <span className={`px-2.5 py-1 rounded-full text-xs font-semibold border ${badgeClass}`}>
                              {rep.status}
                            </span>
                          </td>
                          <td className="py-3.5 px-4 text-right space-x-1 whitespace-nowrap">
                            <button
                              onClick={() => {
                                setSelectedReport(rep);
                                setModalAdminNotes(rep.admin_notes || "");
                              }}
                              className="p-1.5 text-gray-600 hover:bg-gray-100 rounded-lg"
                              title="Lihat Detail"
                            >
                              <Eye className="w-4 h-4" />
                            </button>

                            {rep.status !== "investigating" && (
                              <button
                                onClick={() => handleUpdateStatus(rep.id, "investigating")}
                                className="px-2 py-1 bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-lg text-xs font-medium border border-blue-200"
                              >
                                Investigasi
                              </button>
                            )}

                            {!isBlocked && (
                              <button
                                onClick={() => handleUpdateStatus(rep.id, "resolved_blocked", true)}
                                className="px-2 py-1 bg-red-600 text-white hover:bg-red-700 rounded-lg text-xs font-semibold shadow-sm"
                              >
                                Blokir Subdomain
                              </button>
                            )}

                            {rep.status !== "dismissed" && (
                              <button
                                onClick={() => handleUpdateStatus(rep.id, "dismissed")}
                                className="px-2 py-1 bg-gray-100 text-gray-700 hover:bg-gray-200 rounded-lg text-xs font-medium"
                              >
                                Abaikan
                              </button>
                            )}

                            <button
                              onClick={() => handleDeleteReport(rep.id)}
                              className="p-1.5 text-red-500 hover:bg-red-50 rounded-lg"
                              title="Hapus Laporan"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Tab 2: Blocked Subdomains */}
        {activeTab === "blocked" && (
          <div className="space-y-6">
            {/* Manual Block Form */}
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
              <h3 className="text-lg font-bold text-gray-900 mb-1">{t("manual-block-title")}</h3>
              <p className="text-sm text-gray-500 mb-4">{t("manual-block-desc")}</p>

              <form onSubmit={handleManualBlock} className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Subdomain</label>
                  <input
                    type="text"
                    required
                    placeholder="mis. botnet-target"
                    value={manualSubdomain}
                    onChange={(e) => setManualSubdomain(e.target.value)}
                    className="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-red-500 font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Alasan Pemblokiran</label>
                  <input
                    type="text"
                    placeholder={t("reason-placeholder")}
                    value={manualReason}
                    onChange={(e) => setManualReason(e.target.value)}
                    className="w-full px-3.5 py-2 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-red-500"
                  />
                </div>
                <div className="flex items-end">
                  <button
                    type="submit"
                    className="w-full py-2 bg-red-600 hover:bg-red-700 text-white font-semibold rounded-xl text-sm transition-colors shadow-sm flex items-center justify-center gap-2"
                  >
                    <Ban className="w-4 h-4" />
                    {t("btn-add-block")}
                  </button>
                </div>
              </form>
            </div>

            {/* Blocked List Table */}
            <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
              <div className="p-4 border-b border-gray-100 bg-gray-50/50">
                <h4 className="font-bold text-gray-900 text-sm">Daftar Subdomain Terblokir di Server Otoritatif</h4>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold border-b border-gray-100">
                    <tr>
                      <th className="py-3 px-4">Subdomain</th>
                      <th className="py-3 px-4">Alasan</th>
                      <th className="py-3 px-4">Tanggal Pemblokiran</th>
                      <th className="py-3 px-4 text-right">Aksi</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {blockedList.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="py-10 text-center text-gray-500">
                          Tidak ada subdomain yang sedang diblokir.
                        </td>
                      </tr>
                    ) : (
                      blockedList.map((item) => (
                        <tr key={item.subdomain} className="hover:bg-gray-50">
                          <td className="py-3.5 px-4 font-mono font-bold text-red-600">
                            {item.subdomain}.zcdns.id
                          </td>
                          <td className="py-3.5 px-4 text-gray-700 text-xs">{item.reason}</td>
                          <td className="py-3.5 px-4 text-gray-500 text-xs">
                            {new Date(item.blocked_at).toLocaleString("id-ID")}
                          </td>
                          <td className="py-3.5 px-4 text-right">
                            <button
                              onClick={() => handleUnblock(item.subdomain)}
                              className="px-3 py-1.5 bg-green-50 text-green-700 hover:bg-green-100 rounded-lg text-xs font-semibold border border-green-200 transition-colors"
                            >
                              {t("btn-unblock")}
                            </button>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* Tab 3: System Statistics */}
        {activeTab === "stats" && growthStats && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
              <h3 className="text-lg font-bold text-gray-900 mb-4 flex items-center gap-2">
                <Activity className="w-5 h-5 text-green-600" />
                Telemetri Nyata Pertumbuhan Database
              </h3>
              <div className="space-y-4">
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <span className="text-gray-600 text-sm">Subdomain Aktif / User Session</span>
                  <span className="font-mono font-bold text-gray-900 text-lg">
                    {growthStats.active_subdomains}
                  </span>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <span className="text-gray-600 text-sm">Record DNS Tersimpan</span>
                  <span className="font-mono font-bold text-gray-900 text-lg">
                    {growthStats.active_records}
                  </span>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <span className="text-gray-600 text-sm">Query DNS Terlayani (Log)</span>
                  <span className="font-mono font-bold text-gray-900 text-lg">
                    {growthStats.total_queries}
                  </span>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <span className="text-gray-600 text-sm">Subdomain Pelanggar Diblokir</span>
                  <span className="font-mono font-bold text-red-600 text-lg">
                    {growthStats.blocked_domains}
                  </span>
                </div>
              </div>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
              <h3 className="text-lg font-bold text-gray-900 mb-4 flex items-center gap-2">
                <ShieldCheck className="w-5 h-5 text-blue-600" />
                Status Proteksi Anti-Abuse Server
              </h3>
              <p className="text-sm text-gray-600 mb-4">
                Semua query untuk subdomain yang masuk dalam daftar blokir akan langsung ditolak
                (Response Code: REFUSED) oleh server otoritatif pada interface 10.0.0.10, 10.0.0.11, [fd00::10], dan [fd00::11].
              </p>
              <div className="p-4 bg-green-50 border border-green-200 rounded-xl">
                <div className="flex items-center gap-2 font-semibold text-green-900 text-sm mb-1">
                  <CheckCircle2 className="w-4 h-4 text-green-600" />
                  Anti-Abuse Enforcement Aktif
                </div>
                <p className="text-xs text-green-700">
                  Subdomain yang terverifikasi melakukan spam, malware, phishing, atau konten terlarang dapat langsung dinonaktifkan tanpa downtime server.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Detail Modal */}
        {selectedReport && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="bg-white rounded-2xl max-w-2xl w-full max-h-[90vh] overflow-y-auto shadow-xl border border-gray-200 p-6">
              <div className="flex justify-between items-start mb-4">
                <div>
                  <span className="text-xs font-bold uppercase tracking-wider text-red-600">
                    Detail Laporan Penyalahgunaan
                  </span>
                  <h3 className="text-xl font-extrabold text-gray-900 font-mono mt-1">
                    {selectedReport.subdomain}.zcdns.id
                  </h3>
                </div>
                <button
                  onClick={() => setSelectedReport(null)}
                  className="p-2 text-gray-400 hover:text-gray-600 rounded-lg"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <div className="space-y-4 text-sm">
                <div className="grid grid-cols-2 gap-4 p-4 bg-gray-50 rounded-xl">
                  <div>
                    <span className="text-xs text-gray-500 block">Pelapor</span>
                    <span className="font-semibold text-gray-900">{selectedReport.reporter_name || "-"}</span>
                    <span className="text-xs text-gray-500 block">{selectedReport.reporter_email}</span>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500 block">Tipe Pelanggaran</span>
                    <span className="font-semibold text-red-600 capitalize">{selectedReport.abuse_type}</span>
                    <span className="text-xs text-gray-500 block">
                      Status: <strong className="uppercase">{selectedReport.status}</strong>
                    </span>
                  </div>
                </div>

                <div>
                  <span className="text-xs font-semibold text-gray-700 block mb-1">Deskripsi Pelanggaran:</span>
                  <div className="p-3 bg-gray-50 rounded-xl border border-gray-100 text-gray-800 whitespace-pre-wrap">
                    {selectedReport.description}
                  </div>
                </div>

                {selectedReport.evidence && (
                  <div>
                    <span className="text-xs font-semibold text-gray-700 block mb-1">Bukti Tambahan:</span>
                    <div className="p-3 bg-gray-50 rounded-xl border border-gray-100 text-gray-800 whitespace-pre-wrap font-mono text-xs">
                      {selectedReport.evidence}
                    </div>
                  </div>
                )}

                <div>
                  <label className="text-xs font-semibold text-gray-700 block mb-1">
                    Catatan Internal Administrator:
                  </label>
                  <textarea
                    rows={3}
                    value={modalAdminNotes}
                    onChange={(e) => setModalAdminNotes(e.target.value)}
                    placeholder="Tuliskan catatan investigasi di sini..."
                    className="w-full p-3 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-red-500"
                  />
                </div>

                <div className="flex flex-wrap gap-2 pt-4 border-t border-gray-100 justify-end">
                  <button
                    onClick={() =>
                      handleUpdateStatus(selectedReport.id, "investigating", false, modalAdminNotes)
                    }
                    className="px-4 py-2 bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-xl font-semibold text-xs border border-blue-200"
                  >
                    Tandai Sedang Investigasi
                  </button>

                  <button
                    onClick={() =>
                      handleUpdateStatus(selectedReport.id, "resolved_blocked", true, modalAdminNotes)
                    }
                    className="px-4 py-2 bg-red-600 text-white hover:bg-red-700 rounded-xl font-semibold text-xs shadow-sm flex items-center gap-1.5"
                  >
                    <Ban className="w-3.5 h-3.5" />
                    Blokir Subdomain & Selesaikan
                  </button>

                  <button
                    onClick={() =>
                      handleUpdateStatus(selectedReport.id, "dismissed", false, modalAdminNotes)
                    }
                    className="px-4 py-2 bg-gray-100 text-gray-700 hover:bg-gray-200 rounded-xl font-medium text-xs"
                  >
                    Tolak / Abaikan
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </main>
  );
}
