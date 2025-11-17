import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { CodeComparison } from './CodeComparison';
import logoImage from '../../assets/dingo-logo.png';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/cjs/styles/prism';

interface Example {
  id: number;
  title: string;
  language: string;
  description: string;
  before: string;
  after: string;
  category?: string;
  subcategory?: string;
  summary?: string;
  complexity?: string;
  order?: number;
  reasoning?: string;
}

interface AppProps {
  examples: Example[];
}

// Mock data removed - now using real Dingo examples passed as props

// Group examples by category only (flatten subcategories)
function groupExamples(examples: Example[]) {
  const grouped: Record<string, Example[]> = {};

  examples.forEach(example => {
    const category = example.category || 'Other';

    if (!grouped[category]) {
      grouped[category] = [];
    }
    grouped[category].push(example);
  });

  // Sort examples within each category by subcategory, then by order
  Object.keys(grouped).forEach(category => {
    grouped[category].sort((a, b) => {
      // First sort by subcategory
      const subCatA = a.subcategory || 'ZZZ'; // Put undefined at end
      const subCatB = b.subcategory || 'ZZZ';
      if (subCatA !== subCatB) {
        return subCatA.localeCompare(subCatB);
      }
      // Then sort by order
      return (a.order || 999) - (b.order || 999);
    });
  });

  return grouped;
}

export default function App({ examples }: AppProps) {
  const [selectedId, setSelectedId] = useState(1);

  const groupedExamples = groupExamples(examples);

  // Auto-expand all categories by default
  const allCategories = Object.keys(groupedExamples);

  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(allCategories));

  const selectedExample = examples.find(ex => ex.id === selectedId) || examples[0];

  const toggleCategory = (category: string) => {
    setExpandedCategories(prev => {
      const newSet = new Set(prev);
      if (newSet.has(category)) {
        newSet.delete(category);
      } else {
        newSet.add(category);
      }
      return newSet;
    });
  };

  return (
    <div className="flex h-screen bg-white">
      {/* Sidebar */}
      <div className="w-80 bg-white border-r border-gray-200 flex flex-col">
        <div className="px-6 pt-6 h-20 flex items-center gap-3 relative overflow-visible">
          <img src={logoImage.src} alt="Dingo Logo" className="h-24 w-24 object-contain" />
          <h1 className="text-gray-900">Dingo</h1>
        </div>
        
        {/* Fix #4: Add aria-label to nav */}
        <nav
          aria-label="Example categories navigation"
          className="flex-1 px-6 pt-6 pb-6 space-y-1 overflow-auto"
        >
          {Object.entries(groupedExamples).map(([category, categoryExamples]) => {
            const categoryId = category.toLowerCase().replace(/\s+/g, '-');
            const isExpanded = expandedCategories.has(category);

            return (
              <div key={category} className="mb-3">
                {/* Step 2: Updated category header structure */}
                <button
                  onClick={() => toggleCategory(category)}
                  className="w-full flex items-center justify-between px-3 py-2 text-sm text-gray-900 hover:bg-gray-50 rounded-lg transition-colors group"
                  aria-expanded={isExpanded}
                  aria-controls={`category-${categoryId}`}
                  aria-label={`${isExpanded ? 'Collapse' : 'Expand'} ${category} category with ${categoryExamples.length} examples`}
                >
                  <span className="font-medium">{category}</span>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-400">{categoryExamples.length}</span>
                    {isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-gray-400 transition-transform" aria-hidden="true" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-gray-400 transition-transform" aria-hidden="true" />
                    )}
                  </div>
                </button>

                {/* Step 3: Updated collapse animation container */}
                <div
                  id={`category-${categoryId}`}
                  aria-hidden={!isExpanded}
                  className={`overflow-hidden transition-all duration-300 ease-in-out ${
                    isExpanded ? 'max-h-[2000px] opacity-100 mt-1' : 'max-h-0 opacity-0'
                  }`}
                >
                  <div className="space-y-1 pl-2">
                      {categoryExamples.map((example) => {
                        const isSelected = selectedId === example.id;

                        // Build complete class name for Tailwind (dynamic classes don't work)
                        let buttonClasses = 'w-full text-left px-3 py-2.5 rounded-lg transition-all text-xs ';

                        if (isSelected) {
                          // Apply difficulty-based colors for selected items
                          if (example.complexity === 'basic') {
                            buttonClasses += 'bg-green-50 text-green-700';
                          } else if (example.complexity === 'intermediate') {
                            buttonClasses += 'bg-amber-50 text-amber-700';
                          } else if (example.complexity === 'advanced') {
                            buttonClasses += 'bg-red-50 text-red-700';
                          } else {
                            buttonClasses += 'bg-blue-50 text-blue-700';
                          }
                        } else {
                          buttonClasses += 'text-gray-600 hover:bg-gray-50';
                        }

                        return (
                          <button
                            key={example.id}
                            onClick={() => setSelectedId(example.id)}
                            className={buttonClasses}
                            title={example.summary || example.title}
                            aria-label={`${example.title}${
                              example.complexity ? `, ${example.complexity} complexity` : ''
                            }${isSelected ? ', currently selected' : ''}`}
                            aria-current={isSelected ? 'true' : undefined}
                          >
                            <span className="leading-relaxed">{example.title}</span>
                          </button>
                        );
                      })}
                  </div>
                </div>
              </div>
            );
          })}
        </nav>

        {/* Manifesto excerpt section */}
        <a
          href="/manifesto"
          className="block p-6 border-t border-gray-200 bg-gradient-to-br from-blue-50 to-indigo-50 hover:from-blue-100 hover:to-indigo-100 transition-all cursor-pointer group"
        >
          <div className="flex items-start justify-between mb-2">
            <h3 className="text-gray-900 text-sm font-semibold">The Dingo Manifesto</h3>
            <span className="text-blue-600 text-xs group-hover:translate-x-1 transition-transform">→</span>
          </div>
          <p className="text-gray-700 text-xs leading-relaxed mb-3 italic">
            "Go Broke Free. Are You Ready?"
          </p>
          <p className="text-gray-600 text-xs leading-relaxed">
            You love Go. But you've typed <code className="bg-white px-1 py-0.5 rounded text-xs">if err != nil</code> for the 47th time and thought: "There has to be a better way."
          </p>
          <p className="text-blue-600 text-xs mt-3 font-medium group-hover:underline">
            Read the full manifesto →
          </p>
        </a>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden bg-gray-50">
        
        <div className="flex-1 overflow-auto pt-8">
          <CodeComparison
            before={selectedExample.before}
            after={selectedExample.after}
            language={selectedExample.language}
          />
          
          {/* Reasoning content for this example */}
          <div className="p-8 bg-white">
            <div className="max-w-4xl mx-auto markdown-content">
              {selectedExample.reasoning || selectedExample.description ? (
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={{
                    h1: ({node, ...props}) => <h1 className="mt-4 mb-2 text-sm" {...props} />,
                    h2: ({node, ...props}) => <h2 className="mt-4 mb-2 text-sm" {...props} />,
                    h3: ({node, ...props}) => <h3 className="mt-3 mb-1 text-xs" {...props} />,
                    h4: ({node, ...props}) => <h4 className="mt-3 mb-1 text-xs" {...props} />,
                    p: ({node, ...props}) => <p className="mb-2 text-gray-700 leading-relaxed text-xs" {...props} />,
                    ul: ({node, ...props}) => <ul className="mb-2 ml-4 list-disc text-gray-700 text-xs" {...props} />,
                    ol: ({node, ...props}) => <ol className="mb-2 ml-4 list-decimal text-gray-700 text-xs" {...props} />,
                    li: ({node, ...props}) => <li className="mb-1 text-xs" {...props} />,
                    code: ({node, className, children, ...props}: any) => {
                      const inline = !className;
                      const match = /language-(\w+)/.exec(className || '');
                      const language = match ? match[1] : '';

                      if (!inline && language) {
                        // Block code with language - use syntax highlighter
                        return (
                          <SyntaxHighlighter
                            language={language === 'dingo' ? 'go' : language}
                            style={vscDarkPlus}
                            customStyle={{
                              margin: '0.5rem 0',
                              padding: '1rem',
                              borderRadius: '0.375rem',
                              fontSize: '0.75rem',
                              lineHeight: '1.5',
                            }}
                            showLineNumbers={false}
                            PreTag="div"
                          >
                            {String(children).replace(/\n$/, '')}
                          </SyntaxHighlighter>
                        );
                      }

                      // Inline code or code without language
                      return inline ? (
                        <code className="inline-block bg-gray-100 px-1 py-0.5 rounded text-xs text-gray-800" {...props}>
                          {children}
                        </code>
                      ) : (
                        <code className="bg-gray-100 p-2 rounded text-xs overflow-x-auto block" {...props}>
                          {children}
                        </code>
                      );
                    },
                    pre: ({node, ...props}) => <pre className="mb-2 bg-gray-100 p-2 rounded overflow-x-auto text-xs" {...props} />,
                    a: ({node, ...props}) => <a className="text-blue-600 hover:underline text-xs" {...props} />,
                    blockquote: ({node, ...props}) => <blockquote className="border-l-4 border-gray-300 pl-3 italic text-gray-600 mb-2 text-xs" {...props} />,
                    img: ({node, ...props}) => <img className="max-w-full h-auto rounded my-2" {...props} />,
                    hr: ({node, ...props}) => <hr className="my-4 border-gray-200" {...props} />,
                    table: ({node, ...props}) => <table className="mb-2 border-collapse w-full text-xs" {...props} />,
                    thead: ({node, ...props}) => <thead className="bg-gray-50" {...props} />,
                    tbody: ({node, ...props}) => <tbody {...props} />,
                    tr: ({node, ...props}) => <tr className="border-b border-gray-200" {...props} />,
                    th: ({node, ...props}) => <th className="px-2 py-1 text-left text-xs" {...props} />,
                    td: ({node, ...props}) => <td className="px-2 py-1 text-gray-700 text-xs" {...props} />,
                  }}
                >
                  {selectedExample.reasoning || selectedExample.description}
                </ReactMarkdown>
              ) : (
                <p className="text-gray-500 text-xs">No reasoning documentation available for this example.</p>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}