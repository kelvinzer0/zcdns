import re

with open("src/components/doc-page-client.tsx", "r") as f:
    content = f.read()

# Replace the article tag classes
old_classes = """<article className="prose prose-lg max-w-none 
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
  prose-em:text-gray-700">"""

new_classes = """<article className="prose max-w-none 
  font-serif text-[18px] md:text-[20px] leading-[1.6] md:leading-[32px] text-[#242424]
  prose-headings:font-sans prose-headings:font-bold prose-headings:text-[#242424] prose-headings:tracking-tight prose-headings:mb-4
  prose-h1:text-[32px] md:prose-h1:text-[42px] prose-h1:leading-[1.2]
  prose-h2:text-[26px] md:prose-h2:text-[34px] prose-h2:leading-[1.22] prose-h2:mt-12
  prose-h3:text-[22px] md:prose-h3:text-[26px] prose-h3:leading-[1.22] prose-h3:mt-10
  prose-p:mb-8 prose-p:text-[#242424]
  prose-a:text-[#242424] prose-a:underline prose-a:decoration-1 prose-a:underline-offset-2 hover:prose-a:text-black
  prose-blockquote:border-l-4 prose-blockquote:border-[#242424] prose-blockquote:pl-6 prose-blockquote:italic prose-blockquote:text-gray-600 prose-blockquote:font-serif
  prose-code:text-[#242424] prose-code:bg-gray-100 prose-code:px-2 prose-code:py-1 prose-code:rounded prose-code:text-sm prose-code:font-mono prose-code:font-medium
  prose-pre:bg-[#f9f9f9] prose-pre:text-[#242424] prose-pre:rounded-none prose-pre:border prose-pre:border-gray-200 prose-pre:p-6 prose-pre:text-[14px] md:prose-pre:text-[16px] prose-pre:font-mono
  prose-ul:my-6 prose-ol:my-6 prose-li:my-2 prose-li:text-[#242424]
  prose-strong:font-bold prose-strong:text-[#242424]
  prose-img:my-10 prose-img:rounded-md prose-img:shadow-sm">"""

content = content.replace(old_classes, new_classes)

with open("src/components/doc-page-client.tsx", "w") as f:
    f.write(content)
