import dingoMascot1x from '../../assets/dingo-mascot-peek.png';
import dingoMascot2x from '../../assets/dingo-mascot-peek@2x.png';
import golangMascot1x from '../../assets/golang-mascot-peek.png';
import golangMascot2x from '../../assets/golang-mascot-peek@2x.png';
import { AnimatedMascot } from './AnimatedMascot';

interface CodeComparisonProps {
  beforeHtml: string;
  afterHtml: string;
}

function CodeBlock({
  html,
  mascotSrc,
  mascotSrcSet,
  mascotAlt,
}: {
  html: string;
  mascotSrc: string;
  mascotSrcSet?: string;
  mascotAlt: string;
}) {
  return (
    <div className="relative overflow-visible min-w-0">
      {/* Animated Mascot - Can peek outside container */}
      <AnimatedMascot
        src={mascotSrc}
        srcSet={mascotSrcSet}
        alt={mascotAlt}
        leftPosition={8}
        topPosition={12}
        size={80}
        peekDuration={15}
        scaleOnPeek={1.26}
        scaleOnHide={1.008}
      />

      {/* Code block with rounded corners and clipped overflow */}
      <div className="bg-[#1e1e1e] rounded-xl overflow-hidden shadow-2xl relative z-10">
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
    </div>
  );
}

export function CodeComparison({ beforeHtml, afterHtml }: CodeComparisonProps) {
  return (
    <div className="w-full bg-gray-50">
      <div className="max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-2 gap-6 lg:gap-8 p-4 sm:p-6 lg:p-10">
        {/* Dingo */}
        <div className="flex flex-col gap-3 sm:gap-4 min-w-0">
          <div className="flex items-center gap-2 pl-16 sm:pl-20 lg:pl-24">
            <h3 className="text-lg font-semibold text-gray-800">Dingo</h3>
          </div>
          <CodeBlock
            html={beforeHtml}
            mascotSrc={dingoMascot1x.src}
            mascotSrcSet={`${dingoMascot1x.src} 1x, ${dingoMascot2x.src} 2x`}
            mascotAlt="Dingo mascot peeking"
          />
        </div>

        {/* Goal (Go) */}
        <div className="flex flex-col gap-3 sm:gap-4 min-w-0">
          <div className="flex items-center gap-2 pl-16 sm:pl-20 lg:pl-24">
            <h3 className="text-lg font-semibold text-gray-800">Go</h3>
          </div>
          <CodeBlock
            html={afterHtml}
            mascotSrc={golangMascot1x.src}
            mascotSrcSet={`${golangMascot1x.src} 1x, ${golangMascot2x.src} 2x`}
            mascotAlt="Go Gopher peeking"
          />
        </div>
      </div>
    </div>
  );
}
