import { motion } from "framer-motion";
import { useState } from "react";

interface AnimatedMascotProps {
  src: string;
  srcSet?: string;
  sizes?: string;
  alt: string;
  leftPosition?: number;
  topPosition?: number;
  size?: number;
  peekDuration?: number;
  scaleOnPeek?: number;
  scaleOnHide?: number;
}

export function AnimatedMascot({
  src,
  srcSet,
  sizes,
  alt,
  leftPosition = 8,
  topPosition = 0,
  size = 20,
  peekDuration = 15,
  scaleOnPeek = 1.26,
  scaleOnHide = 1.008,
}: AnimatedMascotProps) {
  // Generate random peek timings for organic behavior
  const [randomParams] = useState(() => {
    const peekTimes: number[] = [];

    // Generate 1-2 random peek moments in the cycle
    const numPeeks = Math.floor(Math.random() * 2) + 1;

    for (let i = 0; i < numPeeks; i++) {
      peekTimes.push(Math.random());
    }

    peekTimes.sort();

    return {
      peekTimes,
      rotations: peekTimes.map(() => Math.random() * 20 - 10),
      // When peeking, show only 30-40% (so hide 60-70%)
      peekAmounts: peekTimes.map(() => Math.random() * 10 + 10), // 10 to 20px (peek just above edge)
    };
  });

  // Create peek animation keyframes
  const createPeekAnimation = (
    peekTimes: number[],
    peekAmounts: number[],
    rotations: number[],
  ) => {
    const keyframes: number[] = [];
    const yValues: number[] = [];
    const rotateValues: number[] = [];
    const scaleValues: number[] = [];

    // Start hidden behind (above the panel)
    keyframes.push(0);
    yValues.push(-70); // Hidden above/behind
    rotateValues.push(0);
    scaleValues.push(scaleOnHide);

    // Add peek moments
    peekTimes.forEach((time, i) => {
      // Just before peek - still hidden
      keyframes.push(Math.max(0, time - 0.02));
      yValues.push(-70);
      rotateValues.push(0);
      scaleValues.push(scaleOnHide);

      // Peek out - slide down to show head
      keyframes.push(time);
      yValues.push(peekAmounts[i]); // Slide down to peek
      rotateValues.push(rotations[i]);
      scaleValues.push(scaleOnPeek);

      // Stay visible briefly
      keyframes.push(Math.min(1, time + 0.05));
      yValues.push(peekAmounts[i]);
      rotateValues.push(rotations[i]);
      scaleValues.push(scaleOnPeek);

      // Hide again
      keyframes.push(Math.min(1, time + 0.08));
      yValues.push(-70);
      rotateValues.push(0);
      scaleValues.push(scaleOnHide);
    });

    // End hidden
    if (keyframes[keyframes.length - 1] < 1) {
      keyframes.push(1);
      yValues.push(-70);
      rotateValues.push(0);
      scaleValues.push(scaleOnHide);
    }

    return { keyframes, yValues, rotateValues, scaleValues };
  };

  const animation = createPeekAnimation(
    randomParams.peekTimes,
    randomParams.peekAmounts,
    randomParams.rotations,
  );

  return (
    <motion.div
      className="absolute pointer-events-none z-0"
      style={{
        left: `${leftPosition}px`,
        top: `${topPosition}px`,
        width: `${size}px`,
        height: `${size}px`,
      }}
      initial={{ y: -70 }}
      animate={{
        y: animation.yValues,
      }}
      transition={{
        duration: peekDuration,
        repeat: Infinity,
        ease: "easeInOut",
        times: animation.keyframes,
      }}
    >
      <motion.img
        src={src}
        srcSet={srcSet}
        sizes={sizes}
        alt={alt}
        className="w-full h-full object-contain"
        initial={{ rotate: 0, scale: scaleOnHide }}
        animate={{
          rotate: animation.rotateValues,
          scale: animation.scaleValues,
        }}
        transition={{
          duration: peekDuration,
          repeat: Infinity,
          ease: "easeInOut",
          times: animation.keyframes,
        }}
      />
    </motion.div>
  );
}
