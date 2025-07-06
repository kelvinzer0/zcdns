import { AlertTriangle, Mail, Shield, ExternalLink } from "lucide-react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card" // Adjusted import path
import { Input } from "./ui/input" // Adjusted import path
import { Label } from "./ui/label" // Adjusted import path
import { Textarea } from "./ui/textarea" // Adjusted import path
import { useTranslations } from '../lib/useTranslations'; // Adjusted import path for placeholder
import { Seo } from './Seo';

export function AbusePage() {
    const t = useTranslations('AbusePage');
    return (
        <main className="pt-20 px-4 sm:px-6 lg:px-8">
            <Seo
                title="Abuse Report - ZeroCentDNS"
                description="Report any misuse of ZeroCentDNS services here. We take abuse reports seriously and have a clear process for investigating and taking action on violations of our policies."
            />
            <div className="container mx-auto max-w-4xl py-8">
                <div className="space-y-8">
                    {/* Header */}
                    <div className="text-center space-y-4">
                        <div className="flex justify-center">
                            <div className="p-3 bg-red-100 ">
                                <AlertTriangle className="h-8 w-8 text-red-600" />
                            </div>
                        </div>
                        <h1 className="text-3xl font-bold text-gray-900">{t('report-abuse')}</h1>
                        <p className="text-gray-600 max-w-2xl mx-auto">
                            {t('helpuse-description')}
                        </p>
                    </div>

                    {/* Report Form */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2">
                                <Shield className="h-5 w-5 text-green-600" />
                                {t('abuse-report-form')}
                            </CardTitle>
                            <CardDescription>
                                {t('abuse-report-description')}
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-6">
                            <form id="abuse-form" className="space-y-6">
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                    <div className="space-y-2">
                                        <Label htmlFor="reporter-name">{t('your-name')}</Label>
                                        <Input
                                            id="reporter-name"
                                            name="reporterName"
                                            placeholder={t('enter-your-full-name')}
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="reporter-email">{t('your-email')}</Label>
                                        <Input
                                            id="reporter-email"
                                            name="reporterEmail"
                                            type="email"
                                            placeholder="your.email@example.com"
                                        />
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="abuse-type">{t('type-of-abuse')}</Label>
                                    <select
                                        name="abuseType"
                                        id="abuse-type"
                                        className="flex h-10 w-full  border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
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
                                    <Label htmlFor="subdomain">{t('subdomain-in-question')}</Label>
                                    <Input
                                        id="subdomain"
                                        name="subdomain"
                                        placeholder="example.zcdns.id"
                                        className="font-mono"
                                    />
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="description">{t('detailed-description')}</Label>
                                    <Textarea
                                        id="description"
                                        name="description"
                                        placeholder={t('detailed-info')}
                                        className="min-h-[120px]"
                                    />
                                </div>

                                <div className="space-y-2">
                                    <Label htmlFor="evidence">{t('additional-evidence-urls-screenshots-etc')}</Label>
                                    <Textarea
                                        id="evidence"
                                        name="evidence"
                                        placeholder={t('additionsal-description')}
                                        className="min-h-[80px]"
                                    />
                                </div>

                                <div className="flex flex-col sm:flex-row gap-4">
                                    <button
                                        type="button"
                                        id="submit-button"
                                        className="inline-flex items-center justify-center whitespace-nowrap  text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-red-600 text-white hover:bg-red-700 h-10 px-4 py-2 flex-1"
                                    >
                                        <AlertTriangle className="w-4 h-4 mr-2" />
                                        {t('submit-abuse-report')}
                                    </button>
                                    <button
                                        type="reset"
                                        className="inline-flex items-center justify-center whitespace-nowrap  text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 border border-input bg-background hover:bg-accent hover:text-accent-foreground h-10 px-4 py-2 flex-1"
                                    >
                                        {t('clear-form')}
                                    </button>
                                </div>
                            </form>
                        </CardContent>
                    </Card>

                    {/* Contact Information */}
                    <Card>
                        <CardHeader>
                            <CardTitle className="flex items-center gap-2">
                                <Mail className="h-5 w-5 text-green-600" />
                                {t('alternative-contact-methods')}
                            </CardTitle>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <div className="grid grid-cols-1 gap-6">
                                <div className="p-4 bg-purple-50 ">
                                    <h3 className="font-semibold text-green-900 mb-2">{t('codeberg-issue')}</h3>
                                    <p className="text-sm text-green-700 mb-3">{t('report-through-our-public-issue-tracker')}</p>
                                    <a
                                        href="https://codeberg.org/zcdns/abuse-reports/issues/new/choose"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1 text-green-600 hover:text-green-800 font-medium"
                                    >
                                        {t('create-issue')}
                                        <ExternalLink className="w-3 h-3" />
                                    </a>
                                    <p className="text-xs text-green-600 mt-1">{t('select-subdomain-abuse-report-template')}</p>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </div>
            </div>
        </main>



    )
}
