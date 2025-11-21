import { useState, useEffect, useRef } from 'react';
import { ChevronDown, ChevronRight, Menu, X } from 'lucide-react';
import { CodeComparison } from './CodeComparison';
import CommentSection from './CommentSection';
import logoImage from '../../assets/dingo-logo.png';

interface Example {
  id: number;
  title: string;
  language: string;
  description: string;
  // Pre-rendered HTML from server (Shiki + marked)
  beforeHtml: string;
  afterHtml: string;
  reasoningHtml?: string | null;
  category?: string;
  subcategory?: string;
  summary?: string;
  complexity?: string;
  order?: number;
  slug?: string; // URL-friendly identifier
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
  // Find the Complete API Server showcase by slug (most reliable)
  const apiServerExample = examples.find(ex => ex.slug === 'showcase_01_api_server');
  const defaultId = apiServerExample?.id || 1;

  const [selectedId, setSelectedId] = useState(defaultId);
  const [isMobileNavOpen, setIsMobileNavOpen] = useState(false);
  const hasReadHash = useRef(false);

  const groupedExamples = groupExamples(examples);

  // Read initial hash on mount (only once, when examples are available)
  useEffect(() => {
    if (hasReadHash.current || examples.length === 0) return;

    const hash = window.location.hash.slice(1);
    if (hash) {
      const example = examples.find(ex => ex.slug === hash);
      if (example) {
        setSelectedId(example.id);
      }
    }
    hasReadHash.current = true;
  }, [examples]); // Run when examples change, but only once thanks to ref

  // Update URL hash when selection changes
  useEffect(() => {
    const selectedExample = examples.find(ex => ex.id === selectedId);
    if (selectedExample?.slug) {
      window.history.replaceState(null, '', `#${selectedExample.slug}`);
    }
  }, [selectedId, examples]);

  // Listen for hash changes (e.g., browser back/forward)
  useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.slice(1);
      if (hash) {
        const example = examples.find(ex => ex.slug === hash);
        if (example) {
          setSelectedId(example.id);
        }
      } else {
        setSelectedId(defaultId);
      }
    };

    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, [examples, defaultId]);

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

  useEffect(() => {
    setIsMobileNavOpen(false);
  }, [selectedId]);

  const renderNavigation = (onExampleSelect?: () => void) => (
    <>
      <div className="px-6 pt-6 h-20 flex items-center gap-3 relative overflow-visible">
        <img src={logoImage.src} alt="Dingo Logo" className="h-16 w-16 object-contain" />
        <div className="flex flex-col">
          <h1 className="text-gray-900">Dingo</h1>
          <p className="text-xs text-gray-500">Pick an example to explore</p>
        </div>
      </div>

      <nav
        aria-label="Example categories navigation"
        className="flex-1 px-6 pt-4 pb-6 space-y-1 overflow-auto"
      >
        {Object.entries(groupedExamples).map(([category, categoryExamples]) => {
          const categoryId = category.toLowerCase().replace(/\s+/g, '-');
          const isExpanded = expandedCategories.has(category);

          return (
            <div key={category} className="mb-3">
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

              <div
                id={`category-${categoryId}`}
                aria-hidden={!isExpanded}
                className={`overflow-hidden transition-all duration-300 ease-in-out ${isExpanded ? 'max-h-[2000px] opacity-100 mt-1' : 'max-h-0 opacity-0'
                  }`}
              >
                <div className="space-y-1 pl-2">
                  {categoryExamples.map((example) => {
                    const isSelected = selectedId === example.id;

                    let buttonClasses = 'w-full text-left px-3 py-2.5 rounded-lg transition-all text-xs ';

                    if (isSelected) {
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
                        onClick={() => {
                          setSelectedId(example.id);
                          onExampleSelect?.();
                        }}
                        className={buttonClasses}
                        title={example.summary || example.title}
                        aria-label={`${example.title}${example.complexity ? `, ${example.complexity} complexity` : ''
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
    </>
  );

  return (
    <div className="min-h-screen bg-white flex flex-col lg:flex-row">
      {/* Mobile top bar */}
      <div className="lg:hidden sticky top-0 z-30 border-b border-gray-200 bg-white/95 backdrop-blur px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <img src={logoImage.src} alt="Dingo Logo" className="h-11 w-11 object-contain" />
          <div className="flex flex-col">
            <p className="text-sm font-semibold text-gray-900">Dingo Examples</p>
            <span className="text-xs text-gray-500">Preview Dingo ⇄ Go output</span>
          </div>
        </div>
        <button
          onClick={() => setIsMobileNavOpen(true)}
          className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-200 bg-white shadow-sm text-sm font-medium text-gray-800"
          aria-label="Open example navigation"
        >
          <Menu className="w-4 h-4" />
          Browse
        </button>
      </div>

      {/* Mobile drawer */}
      {isMobileNavOpen && (
        <div className="lg:hidden fixed inset-0 z-40">
          <div className="absolute inset-0 bg-black/40" onClick={() => setIsMobileNavOpen(false)} aria-hidden="true" />
          <aside
            className="absolute inset-y-0 left-0 w-[82%] max-w-xs bg-white shadow-2xl flex flex-col"
            role="dialog"
            aria-modal="true"
            aria-label="Example navigation"
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200">
              <div className="flex items-center gap-3">
                <img src={logoImage.src} alt="Dingo Logo" className="h-12 w-12 object-contain" />
                <div className="flex flex-col">
                  <p className="text-sm font-semibold text-gray-900">Pick an example</p>
                  <span className="text-xs text-gray-500">Categories & scenarios</span>
                </div>
              </div>
              <button
                onClick={() => setIsMobileNavOpen(false)}
                className="p-2 rounded-md hover:bg-gray-100 text-gray-500"
                aria-label="Close navigation"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto">
              {renderNavigation(() => setIsMobileNavOpen(false))}
            </div>
          </aside>
        </div>
      )}

      {/* Sidebar for desktop */}
      <aside className="hidden lg:flex w-80 bg-white border-r border-gray-200 flex-shrink-0 flex-col sticky top-0 max-h-screen">
        <div className="flex flex-col h-full">
          {renderNavigation()}
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex-1 flex flex-col bg-gray-50 min-w-0">
        <div className="flex-1">
          <CodeComparison
            beforeHtml={selectedExample.beforeHtml}
            afterHtml={selectedExample.afterHtml}
          />

          {/* Reasoning content for this example (pre-rendered markdown from server) */}
          <div className="px-4 sm:px-6 lg:px-10 pb-10">
            <div className="max-w-5xl mx-auto bg-white rounded-2xl shadow-sm">
              <div className="p-4 sm:p-6 lg:p-8">
                <div className="prose prose-sm prose-gray">
                  {selectedExample.reasoningHtml ? (
                    <div
                      className="markdown-content"
                      dangerouslySetInnerHTML={{ __html: selectedExample.reasoningHtml }}
                    />
                  ) : (
                    <p className="text-gray-500 text-xs">No reasoning documentation available for this example.</p>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Comment Section */}
          <div className="px-4 sm:px-6 lg:px-10 pb-12">
            <div className="max-w-5xl mx-auto">
              <CommentSection slug={selectedExample.slug || `example-${selectedExample.id}`} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
