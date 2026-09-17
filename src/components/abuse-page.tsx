import { useState } from "react";
import { AlertTriangle, Mail, Shield, ExternalLink, CheckCircle, AlertCircle, Loader2 } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { Textarea } from "./ui/textarea";
import { useTranslations } from '../lib/useTranslations';
import { Seo } from './Seo';

export function AbusePage() {
    const t = useTranslations('AbusePage');

    const [reporterName, setReporterName] = useState("");
    const [reporterEmail, setReporterEmail] = useState("");
    const [abuseType, setAbuseType] = useState("");
    const [subdomain, setSubdomain] = useState("");
    const [description, setDescription] = useState("");
    const [evidence, setEvidence] = useState("");

    const [isSubmitting, setIsSubmitting] = useState(false);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);
    const [errorMessage, setErrorMessage] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setErrorMessage(null);
        setSuccessMessage(null);

        if (!reporterEmail || !reporterEmail.includes("@")) {
            setErrorMessage("Harap masukkan alamat email yang valid.");
            return;
        }

        if (!subdomain) {
            setErrorMessage("Harap masukkan subdomain yang dilaporkan.");
            return;
        }

        if (!description) {
            setErrorMessage("Harap berikan deskripsi rinci mengenai penyalahgunaan.");
            return;
        }

        setIsSubmitting(true);

        try {
            const res = await fetch("/api/abuse", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    reporter_name: reporterName,
                    reporter_email: reporterEmail,
                    abuse_type: abuseType || "other",
                    subdomain: subdomain,
                    description: description,
                    evidence: evidence,
                }),
            });

            const data = await res.json();

            if (!res.ok) {
                throw new Error(data.error || "Gagal mengirim laporan penyalahgunaan.");
            }

            setSuccessMessage(
                data.message || "Laporan penyalahgunaan berhasil dikirim. Tim administrator ZCDNS akan meninjau laporan ini secepatnya."
            );

            // Reset form fields
            setReporterName("");
            setReporterEmail("");
            setAbuseType("");
            setSubdomain("");
            setDescription("");
            setEvidence("");
        } catch (err: any) {
            setErrorMessage(err.message || "Terjadi kesalahan saat mengirim laporan. Silakan coba lagi.");
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <main className="pt-20 px-4 sm:px-6 lg:px-8 pb-16">
            <Seo
                title="Abuse Report - ZeroCentDNS"
                description="Report any misuse of ZeroCentDNS services here. We take abuse reports seriously and have a clear process for investigating and taking action on violations of our policies."
            />
            <div className="container mx-auto max-w-4xl py-8">
                <div className="space-y-8">
                    {/* Header */}
                    <div className="text-center space-y-4">
                        <div className="flex justify-center">
                            <div className="p-3 bg-red-100 rounded-2xl">
                                <AlertTriangle className="h-8 w-8 text-red-600" />
                            </div>
                        </div>
                        <h1 className="text-3xl font-bold text-gray-900">{t('report-abuse')}</h1>
                        <p className="text-gray-600 max-w-2xl mx-auto">
                            {t('helpuse-description')}
                        </p>
                    </div>

                    {/* Success Notification */}
                    {successMessage && (
                        <div className="p-4 bg-green-50 border border-green-200 rounded-xl flex items-start gap-3 text-green-800">
                            <CheckCircle className="h-5 w-5 text-green-600 flex-shrink-0 mt-0.5" />
                            <div>
                                <p className="font-semibold text-green-900">Laporan Berhasil Diterima</p>
                                <p className="text-sm mt-1">{successMessage}</p>
                            </div>
                        </div>
                    )}

                    {/* Error Notification */}
                    {errorMessage && (
                        <div className="p-4 bg-red-50 border border-red-200 rounded-xl flex items-start gap-3 text-red-800">
                            <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                            <div>
                                <p className="font-semibold text-red-900">Gagal Mengirim Laporan</p>
                                <p className="text-sm mt-1">{errorMessage}</p>
                            </div>
                        </div>
                    )}

                    {/* Report Form */}
                    <Card className="shadow-sm border-gray-200">
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-xl">
                                <Shield className="h-5 w-5 text-green-600" />
                                {t('abuse-report-form')}
                            </CardTitle>
                            <CardDescription>
                                {t('abuse-report-description')}
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-6">
                            <form id="abuse-form" onSubmit={handleSubmit} className="space-y-6">
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                    <div className="space-y-2">
                                        <Label htmlFor="reporter-name">{t('your-name')}</Label>
                                        <Input
                                            id="reporter-name"
                                            name="reporterName"
                                            value={reporterName}
                                            onChange={(e) => setReporterName(e.target.value)}
                                            placeholder={t('enter-your-full-name')}
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="reporter-email">{t('your-email')} <span className="text-red-500">*</span></Label>
                                        <Input
                                            id="reporter-email"
                                            name="reporterEmail"
                                            type="email"
                                            required
                                            value={reporterEmail}
                                            onChange={(e) => setReporterEmail(e.target.value)}
                                            placeholder="your.email@example.com"
                                        />
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="abuse-type">{t('type-of-abuse')}</Label>
                                    <select
                                        name="abuseType"
                                        id="abuse-type"
                                        value={abuseType}
                                        onChange={(e) => setAbuseType(e.target.value)}
                                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                                    >
                                        <option value="">{t('select-the-type-of-abuse')}</option>
                                        <option value="spam">Spam / Unsolicited Email</option>
                                        <option value="malware">Malware / Phishing</option>
                                        <option value="copyright">Copyright Infringement</option>
                                        <option value="harassment">Harassment / Threats</option>
                                        <option value="illegal">Illegal Content</option>
                                        <option value="commercial">Commercial Use (Violation of Terms)</option>
                                        <option value="other">Other</option>
                                    </select>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="subdomain">{t('subdomain-in-question')} <span className="text-red-500">*</span></Label>
                                    <Input
                                        id="subdomain"
                                        name="subdomain"
                                        required
                                        value={subdomain}
                                        onChange={(e) => setSubdomain(e.target.value)}
                                        placeholder="contoh-subdomain.zcdns.id"
                                        className="font-mono"
                                    />
                                    <p className="text-xs text-gray-500">Masukkan subdomain yang diduga melakukan pelanggaran.</p>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="description">{t('detailed-description')} <span className="text-red-500">*</span></Label>
                                    <Textarea
                                        id="description"
                                        name="description"
                                        required
                                        value={description}
                                        onChange={(e) => setDescription(e.target.value)}
                                        placeholder={t('detailed-info')}
                                        className="min-h-[120px]"
                                    />
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="evidence">{t('additional-evidence-urls-screenshots-etc')}</Label>
                                    <Textarea
                                        id="evidence"
                                        name="evidence"
                                        value={evidence}
                                        onChange={(e) => setEvidence(e.target.value)}
                                        placeholder={t('additionsal-description')}
                                        className="min-h-[80px]"
                                    />
                                </div>

                                <div className="flex flex-col sm:flex-row gap-4 pt-2">
                                    <button
                                        type="submit"
                                        disabled={isSubmitting}
                                        className="inline-flex items-center justify-center rounded-lg whitespace-nowrap text-sm font-semibold transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring bg-red-600 text-white hover:bg-red-700 h-11 px-6 py-2 flex-1 shadow-sm disabled:opacity-50"
                                    >
                                        {isSubmitting ? (
                                            <>
                                                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                                Mengirim Laporan...
                                            </>
                                        ) : (
                                            <>
                                                <AlertTriangle className="w-4 h-4 mr-2" />
                                                {t('submit-abuse-report')}
                                            </>
                                        )}
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => {
                                            setReporterName("");
                                            setReporterEmail("");
                                            setAbuseType("");
                                            setSubdomain("");
                                            setDescription("");
                                            setEvidence("");
                                            setErrorMessage(null);
                                        }}
                                        className="inline-flex items-center justify-center rounded-lg whitespace-nowrap text-sm font-medium transition-colors border border-gray-200 bg-white hover:bg-gray-50 h-11 px-6 py-2"
                                    >
                                        {t('clear-form')}
                                    </button>
                                </div>
                            </form>
                        </CardContent>
                    </Card>

                    {/* Contact Information */}
                    <Card className="shadow-sm border-gray-200">
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2 text-lg">
                                <Mail className="h-5 w-5 text-green-600" />
                                {t('alternative-contact-methods')}
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <div className="grid grid-cols-1 gap-6">
                                <div className="p-4 bg-gray-50 rounded-xl border border-gray-100">
                                    <h3 className="font-semibold text-gray-900 mb-1">{t('codeberg-issue')}</h3>
                                    <p className="text-sm text-gray-600 mb-3">{t('report-through-our-public-issue-tracker')}</p>
                                    <a
                                        href="https://codeberg.org/zcdns/abuse-reports/issues/new/choose"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1.5 text-green-700 hover:text-green-800 font-medium text-sm"
                                    >
                                        {t('create-issue')}
                                        <ExternalLink className="w-3.5 h-3.5" />
                                    </a>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </div>
            </div>
        </main>
    );
}
