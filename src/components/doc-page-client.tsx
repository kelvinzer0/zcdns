import { ChevronRight, MenuSquare, X } from "lucide-react"
import { useEffect, useState } from "react"

import { Button } from "./ui/button"
import { DocSidebar } from "./doc-sidebar"
import { Link } from 'react-router-dom'

interface SidebarItem {
    label: string
    slug?: string
    icon: string
    children?: {
        label: string
        slug: string
        icon: string
    }[]
}

interface DocPageClientProps {
    locale: string
    slug: string[]
    content: string // This will be the raw MDX string
    navigation: {
        previous: { label: string; slug: string } | null
        next: { label: string; slug: string } | null
    }
    sidebarItems: SidebarItem[]
}

export function DocPageClient({
    locale,
    slug,
    content,
    navigation,
    sidebarItems
}: DocPageClientProps) {
    const [isSidebarOpen, setIsSidebarOpen] = useState(false)
    const [searchQuery, setSearchQuery] = useState("")
    const [searchResults, setSearchResults] = useState<{ label: string; slug: string }[]>([])
    const [showSearchResults, setShowSearchResults] = useState(false)

    // Search function (moved here to be passed as prop)
    const searchItems = (query: string, items: SidebarItem[]) => {
        if (!query.trim()) return []

        const allItems: { label: string; slug: string }[] = []

        items.forEach(item => {
            if (item.children) {
                item.children.forEach(child => {
                    allItems.push({ label: child.label, slug: child.slug })
                })
            } else if (item.slug) {
                allItems.push({ label: item.label, slug: item.slug })
            }
        })

        return allItems.filter(item =>
            item.label.toLowerCase().includes(query.toLowerCase()) ||
            item.slug.toLowerCase().includes(query.toLowerCase())
        )
    }

    // Handle search
    useEffect(() => {
        if (searchQuery.trim()) {
            const results = searchItems(searchQuery, sidebarItems)
            setSearchResults(results)
            setShowSearchResults(true)
        } else {
            setSearchResults([])
            setShowSearchResults(false)
        }
    }, [searchQuery, sidebarItems])

    // Close sidebar when clicking outside on mobile
    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            const sidebar = document.getElementById('mobile-sidebar')
            const menuButton = document.getElementById('menu-button')

            if (isSidebarOpen && sidebar && !sidebar.contains(event.target as Node) && !menuButton?.contains(event.target as Node)) {
                setIsSidebarOpen(false)
            }
        }

        document.addEventListener('mousedown', handleClickOutside)
        return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [isSidebarOpen])

    // Close search results when clicking outside
    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            const searchContainer = document.getElementById('search-container')
            if (showSearchResults && searchContainer && !searchContainer.contains(event.target as Node)) {
                setShowSearchResults(false)
            }
        }

        document.addEventListener('mousedown', handleClickOutside)
        return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [showSearchResults])

    return (
        <>
            
            <div className="min-h-screen bg-gray-50 pt-16"> {/* Added pt-16 to account for fixed header */}
                {/* Mobile Docs Sidebar Toggle Button */}
                <div className="lg:hidden fixed top-16 left-0 right-0 bg-white border-b border-gray-200 px-4 py-3 z-30">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="w-full justify-start"
                        id="menu-button"
                        onClick={() => setIsSidebarOpen(!isSidebarOpen)}
                    >
                        {isSidebarOpen ? <X className="h-5 w-5 mr-2" /> : <MenuSquare className="h-5 w-5 mr-2" />}
                        Documentation Menu
                    </Button>
                </div>

                {/* Mobile Sidebar Overlay */}
                {isSidebarOpen && (
                    <div className="lg:hidden fixed inset-0 z-40 bg-opacity-50" />
                )}

                {/* Mobile Sidebar */}
                <aside
                    id="mobile-sidebar"
                    className={`lg:hidden fixed left-0 top-16 h-[calc(100vh-4rem)] w-80 bg-white z-50 transform transition-transform duration-300 ease-in-out ${isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
                        }`}
                >
                    <div className="pt-4">
                        <DocSidebar
                            locale={locale}
                            sidebarItems={sidebarItems}
                            searchQuery={searchQuery}
                            setSearchQuery={setSearchQuery}
                            searchResults={searchResults}
                            showSearchResults={showSearchResults}
                            setShowSearchResults={setShowSearchResults}
                            setIsSidebarOpen={setIsSidebarOpen}
                        />
                    </div>
                </aside>

                <div className="flex">
                    {/* Desktop Sidebar */}
                    <aside className="hidden lg:block w-80 bg-white border-r border-gray-200 fixed left-0 top-16 h-[calc(100vh-4rem)] overflow-y-auto">
                        <DocSidebar
                            locale={locale}
                            sidebarItems={sidebarItems}
                            searchQuery={searchQuery}
                            setSearchQuery={setSearchQuery}
                            searchResults={searchResults}
                            showSearchResults={showSearchResults}
                            setShowSearchResults={setShowSearchResults}
                            setIsSidebarOpen={setIsSidebarOpen}
                        />
                    </aside>

                    {/* Main Content */}
                    <main className="flex-1 lg:ml-80">
                        <div className="max-w-4xl mx-auto pt-0 sm:pt-16">
                            {/* Breadcrumb - Added more top padding for desktop */}
                            <div className="px-6 py-2 lg:py-4 bg-white border-b border-gray-200">
                                <nav className="flex items-center space-x-2 text-sm text-gray-500">
                                    <Link to={`/${locale}/docs/overview`} className="hover:text-blue-600">
                                        Documentation
                                    </Link>
                                    {slug.map((segment, index) => (
                                        <div key={index} className="flex items-center space-x-2">
                                            <ChevronRight className="h-4 w-4" />
                                            <span className={index === slug.length - 1 ? "text-gray-900 font-medium" : "hover:text-blue-600"}>
                                                {segment.charAt(0).toUpperCase() + segment.slice(1)}
                                            </span>
                                        </div>
                                    ))}
                                </nav>
                            </div>

                            {/* Content */}
                            <div className="px-6 py-8">
                                <article className="prose prose-lg max-w-none 
  prose-headings:text-gray-900 prose-headings:font-semibold prose-headings:tracking-tight
  prose-p:text-gray-700 prose-p:leading-7 prose-p:mb-4
  prose-a:text-blue-600 prose-a:no-underline hover:prose-a:underline prose-a:font-medium
  prose-code:text-blue-700 prose-code:bg-blue-50 prose-code:px-2 prose-code:py-1 prose-code: prose-code:text-sm prose-code:font-medium prose-code:break-words prose-code:max-w-full
  prose-pre:bg-gray-900 prose-pre:text-gray-100 prose-pre: prose-pre:overflow-x-auto prose-pre:p-4 prose-pre:max-w-full
  prose-blockquote:border-l-4 prose-blockquote:border-blue-500 prose-blockquote:bg-blue-50 prose-blockquote:p-4 prose-blockquote:-r-lg
  prose-ul:my-4 prose-ol:my-4 prose-li:my-1
  prose-table:w-full prose-table:border-collapse prose-table:border prose-table:border-gray-200 prose-table: prose-table:overflow-hidden
  prose-thead:bg-gray-50 prose-th:border prose-th:border-gray-200 prose-th:px-4 prose-th:py-3 prose-th:text-left prose-th:font-semibold prose-th:text-gray-900
  prose-td:border prose-td:border-gray-200 prose-td:px-4 prose-td:py-3 prose-td:text-gray-700
  prose-hr:border-gray-200 prose-hr:my-8
  prose-strong:text-gray-900 prose-strong:font-semibold
  prose-em:text-gray-700">
                                    <div dangerouslySetInnerHTML={{ __html: content }} />
                                </article>

                                {/* Navigation Footer */}
                                <div className="mt-12 pt-8 border-t border-gray-200">
                                    <div className="flex justify-between items-center">
                                        <div className="flex-1">
                                            {navigation.previous && (
                                                <Link to={`/${locale}/docs/${navigation.previous.slug}`}>
                                                    <Button variant="ghost" className="text-gray-600 hover:text-blue-600">
                                                        ← {navigation.previous.label}
                                                    </Button>
                                                </Link>
                                            )}
                                        </div>
                                        <div className="flex-1 text-right">
                                            {navigation.next && (
                                                <Link to={`/${locale}/docs/${navigation.next.slug}`}>
                                                    <Button variant="ghost" className="text-gray-600 hover:text-blue-600">
                                                        {navigation.next.label} →
                                                    </Button>
                                                </Link>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </main>
                </div>
            </div>
        </>
    )
}