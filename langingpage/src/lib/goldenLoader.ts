// Custom loader for golden test files
// Reads .dingo, .go.golden, and .reasoning.md files from ../tests/golden/

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import matter from 'gray-matter';
import type { Loader } from 'astro/loaders';
import {
  extractFeatureType,
  extractOrder,
  extractBaseName,
  generateTitle,
  featureTypes,
} from './featureMapping';

export interface GoldenExample {
  // Basic identification
  id: string;
  title: string;

  // Legacy fields (backward compatibility)
  featureType: string;
  featureDisplayName: string;

  // Code content
  dingoCode: string;
  goCode: string;
  reasoning?: string;

  // Frontmatter metadata
  category?: string;
  category_order?: number; // Controls category sorting on landing page
  subcategory?: string;
  test_id?: string;
  order: number; // Controls example order within category

  // Classification
  complexity?: 'basic' | 'intermediate' | 'advanced';
  feature?: string;
  phase?: string;
  status?: 'implemented' | 'planned' | 'experimental';

  // Metadata
  description?: string;
  summary?: string;
  code_reduction?: number;
  lines_dingo?: number;
  lines_go?: number;

  // References
  go_proposal?: string;
  go_proposal_link?: string;
  feature_file?: string;
  related_tests?: string[];

  // Context
  tags?: string[];
  keywords?: string[];
}

/**
 * Custom loader to read golden test files from ../tests/golden/
 */
export function goldenLoader(): Loader {
  return {
    name: 'golden-loader',
    load: async ({ store, logger }) => {
      // Resolve path to golden tests directory (relative to this file)
      const currentDir = path.dirname(fileURLToPath(import.meta.url));
      const goldenDir = path.resolve(currentDir, '../../../tests/golden');

      logger.info(`Loading golden examples from: ${goldenDir}`);

      // Check if directory exists
      if (!fs.existsSync(goldenDir)) {
        logger.warn(`Golden tests directory not found: ${goldenDir}`);
        return;
      }

      // Read all files from golden directory
      const allFiles = fs.readdirSync(goldenDir);

      // Group files by base name
      const fileGroups: Map<string, { dingo?: string; go?: string; reasoning?: string }> =
        new Map();

      for (const file of allFiles) {
        if (file.endsWith('.dingo')) {
          const baseName = extractBaseName(file);
          if (!fileGroups.has(baseName)) {
            fileGroups.set(baseName, {});
          }
          fileGroups.get(baseName)!.dingo = file;
        } else if (file.endsWith('.go.golden')) {
          const baseName = extractBaseName(file);
          if (!fileGroups.has(baseName)) {
            fileGroups.set(baseName, {});
          }
          fileGroups.get(baseName)!.go = file;
        } else if (file.endsWith('.reasoning.md')) {
          const baseName = extractBaseName(file);
          if (!fileGroups.has(baseName)) {
            fileGroups.set(baseName, {});
          }
          fileGroups.get(baseName)!.reasoning = file;
        }
      }

      logger.info(`Found ${fileGroups.size} example groups`);

      // Process each group and create collection entries
      for (const [baseName, files] of fileGroups) {
        try {
          // Skip if missing required files (.dingo or .go.golden)
          if (!files.dingo || !files.go) {
            logger.warn(`Skipping incomplete example: ${baseName} (missing .dingo or .go.golden)`);
            continue;
          }

          // Read file contents with error handling
          const dingoCode = fs.readFileSync(path.join(goldenDir, files.dingo), 'utf-8');
          const goCode = fs.readFileSync(path.join(goldenDir, files.go), 'utf-8');
          const reasoningRaw = files.reasoning
            ? fs.readFileSync(path.join(goldenDir, files.reasoning), 'utf-8')
            : undefined;

          // Validate content is not empty
          if (!dingoCode.trim()) {
            logger.warn(`Empty Dingo code in: ${files.dingo} - skipping`);
            continue;
          }

          if (!goCode.trim()) {
            logger.warn(`Empty Go code in: ${files.go} - skipping`);
            continue;
          }

          // Validate content is valid text (not binary)
          if (dingoCode.includes('\0') || goCode.includes('\0')) {
            logger.error(
              `Binary content detected in: ${baseName} (expected text files) - skipping`,
            );
            continue;
          }

          // Parse frontmatter from reasoning file
          let frontmatter: Record<string, any> = {};
          let reasoning: string | undefined = reasoningRaw;
          if (reasoningRaw) {
            try {
              const parsed = matter(reasoningRaw);
              frontmatter = parsed.data;
              reasoning = parsed.content;
            } catch (error) {
              logger.warn(`Failed to parse frontmatter in ${files.reasoning}: ${error}`);
              // Keep original reasoning if frontmatter parsing fails
            }
          }

          // Extract metadata from file name (legacy fallback)
          const featureType = extractFeatureType(baseName);
          const featureDisplayName = featureTypes[featureType] || 'Other';
          const fileNameOrder = extractOrder(baseName);

          // Use frontmatter values with fallbacks
          const title = frontmatter.title || generateTitle(baseName);
          const order = frontmatter.order || fileNameOrder;

          // Create entry with all metadata
          const entry: GoldenExample = {
            // Basic identification
            id: baseName,
            title,

            // Legacy fields (backward compatibility)
            featureType,
            featureDisplayName,

            // Code content
            dingoCode,
            goCode,
            reasoning,

            // Frontmatter metadata
            category: frontmatter.category,
            category_order: frontmatter.category_order,
            subcategory: frontmatter.subcategory,
            test_id: frontmatter.test_id,
            order,

            // Classification
            complexity: frontmatter.complexity,
            feature: frontmatter.feature,
            phase: frontmatter.phase,
            status: frontmatter.status,

            // Metadata
            description: frontmatter.description,
            summary: frontmatter.summary,
            code_reduction: frontmatter.code_reduction,
            lines_dingo: frontmatter.lines_dingo,
            lines_go: frontmatter.lines_go,

            // References
            go_proposal: frontmatter.go_proposal,
            go_proposal_link: frontmatter.go_proposal_link,
            feature_file: frontmatter.feature_file,
            related_tests: frontmatter.related_tests,

            // Context
            tags: frontmatter.tags,
            keywords: frontmatter.keywords,
          };

          // Store entry in collection
          store.set({
            id: baseName,
            data: entry as unknown as Record<string, unknown>,
          });

          logger.info(`Loaded example: ${baseName} (${featureDisplayName})`);
        } catch (error) {
          // Catch and log file read errors without crashing the build
          const errorMessage = error instanceof Error ? error.message : String(error);
          logger.error(`Failed to load example ${baseName}: ${errorMessage}`);
          continue;
        }
      }

      logger.info('Golden examples loaded successfully');
    },
  };
}
