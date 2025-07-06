import { BookOpen, FileText, Folder, Search } from "lucide-react"
import type { Dispatch, SetStateAction } from "react"

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

interface DocSidebarProps {
    locale: string
    sidebarItems: SidebarItem[]
    searchQuery: string
    setSearchQuery: Dispatch<SetStateAction<string>>
    searchResults: { label: string; slug: string }[]
    showSearchResults: boolean
    setShowSearchResults: Dispatch<SetStateAction<boolean>>
    setIsSidebarOpen: Dispatch<SetStateAction<boolean>>
}

// Icon mapping
const iconMap = {
    BookOpen,
    FileText,
    Folder,
    Search,
}

export function DocSidebar({
    locale,
    sidebarItems,
    searchQuery,
    setSearchQuery,
    searchResults,
    showSearchResults,
    setShowSearchResults,
    setIsSidebarOpen,
}: DocSidebarProps) {
    return (
        <div className="p-6">
            {/* Search Bar */}
            <div className="relative mb-6" id="search-container">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                    type="text"
                    placeholder="Search documentation..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onFocus={() => searchQuery.trim() && setShowSearchResults(true)}
                    className="w-full pl-10 pr-4 py-2 border border-gray-200  focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent text-sm"
                />

                {/* Search Results */}
                {showSearchResults && (
                    <div className="absolute top-full left-0 right-0 mt-2 bg-white border border-gray-200  shadow-lg z-50 max-h-60 overflow-y-auto">
                        {searchResults.length > 0 ? (
                            searchResults.map((result, index) => (
                                <Link
                                    key={index}
                                    to={`/${locale}/docs/${result.slug}`}
                                    className="block px-4 py-2 text-sm text-gray-700 hover:bg-blue-50 hover:text-blue-600 border-b border-gray-100 last:border-b-0"
                                    onClick={() => {
                                        setSearchQuery("")
                                        setShowSearchResults(false)
                                        setIsSidebarOpen(false)
                                    }}
                                >
                                    {result.label}
                                </Link>
                            ))
                        ) : (
                            <div className="px-4 py-2 text-sm text-gray-500">
                                No results found for "{searchQuery}"
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Navigation */}
            <nav className="space-y-1">
                {sidebarItems.map((item, i) => {
                    const IconComponent = iconMap[item.icon as keyof typeof iconMap]

                    return (
                        <div key={i}>
                            {item.children ? (
                                <div className="space-y-1">
                                    <div className="flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-gray-50 ">
                                        <IconComponent className="mr-3 h-4 w-4 text-gray-500" />
                                        {item.label}
                                    </div>
                                    <div className="ml-6 space-y-1">
                                        {item.children.map((sub, j) => {
                                            const SubIconComponent = iconMap[sub.icon as keyof typeof iconMap]
                                            return (
                                                <Link
                                                    key={j}
                                                    to={`/${locale}/docs/${sub.slug}`}
                                                    className="flex items-center px-3 py-2 text-sm text-gray-600 hover:text-blue-600 hover:bg-blue-50  transition-colors duration-150"
                                                    onClick={() => setIsSidebarOpen(false)}
                                                >
                                                    <SubIconComponent className="mr-3 h-4 w-4" />
                                                    {sub.label}
                                                </Link>
                                            )
                                        })}
                                    </div>
                                </div>
                            ) : (
                                item.slug && (
                                    <Link
                                        to={`/${locale}/docs/${item.slug}`}
                                        className="flex items-center px-3 py-2 text-sm font-medium text-gray-700 hover:text-blue-600 hover:bg-blue-50  transition-colors duration-150"
                                        onClick={() => setIsSidebarOpen(false)}
                                    >
                                        <IconComponent className="mr-3 h-4 w-4" />
                                        {item.label}
                                    </Link>
                                )
                            )}
                        </div>
                    )
                })}
            </nav>
        </div>
    )
}
