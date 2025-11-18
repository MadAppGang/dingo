// Retina image handling: 1x and 2x versions for sharp display on all screens
import dingoLogo1x from '../../assets/dingo-logo-small.png';
import dingoLogo2x from '../../assets/dingo-logo-2x.png';
import golangLogo1x from '../../assets/golang-logo-small.png';
import golangLogo2x from '../../assets/golang-logo-2x.png';

interface CodeComparisonProps {
  beforeHtml: string;
  afterHtml: string;
}

function CodeBlock({ html }: { html: string }) {
  return (
    <div className="bg-[#1e1e1e] rounded-xl overflow-hidden shadow-2xl">
      {/* macOS-style window controls */}
      <div className="bg-[#323232] px-4 py-3 flex items-center gap-2">
        <div className="w-3 h-3 rounded-full bg-[#ff5f56]"></div>
        <div className="w-3 h-3 rounded-full bg-[#ffbd2e]"></div>
        <div className="w-3 h-3 rounded-full bg-[#27c93f]"></div>
      </div>

      {/* Pre-rendered code with syntax highlighting (from Shiki) */}
      <div
        className="overflow-auto shiki-code"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </div>
  );
}

export function CodeComparison({ beforeHtml, afterHtml }: CodeComparisonProps) {
  return (
    <div className="grid grid-cols-2 gap-8 p-8">
      {/* Dingo */}
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <div className="w-12 h-12 flex items-center justify-center">
            <img
              src={dingoLogo1x.src}
              srcSet={`${dingoLogo1x.src} 1x, ${dingoLogo2x.src} 2x`}
              alt="Dingo logo"
              className="w-12 h-12 object-contain rounded-lg"
              width={48}
              height={48}
            />
          </div>
          <h3 className="text-lg font-semibold text-gray-800">Dingo</h3>
        </div>
        <CodeBlock html={beforeHtml} />
      </div>

      {/* Goal (Go) */}
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <div className="w-12 h-12 flex items-center justify-center">
            <img
              src={golangLogo1x.src}
              srcSet={`${golangLogo1x.src} 1x, ${golangLogo2x.src} 2x`}
              alt="Go logo"
              className="w-12 h-12 object-contain"
              width={48}
              height={48}
            />
          </div>
          <h3 className="text-lg font-semibold text-gray-800">Go</h3>
        </div>
        <CodeBlock html={afterHtml} />
      </div>
    </div>
  );
}