// Content collections configuration

import { defineCollection, z } from 'astro:content';
import { goldenLoader } from './lib/goldenLoader';

// Define schema for golden examples
const goldenExamples = defineCollection({
  loader: goldenLoader(),
  schema: z.object({
    // Basic identification
    id: z.string(),
    title: z.string(),

    // Legacy fields (for backward compatibility)
    featureType: z.string(),
    featureDisplayName: z.string(),

    // Code content
    dingoCode: z.string(),
    goCode: z.string(),
    reasoning: z.string().optional(),

    // New frontmatter metadata
    category: z.string().optional(),
    category_order: z.number().optional(), // Controls category sorting on landing page
    subcategory: z.string().optional(),
    test_id: z.string().optional(),
    order: z.number(), // Controls example order within category

    // Classification
    complexity: z.enum(['basic', 'intermediate', 'advanced']).optional(),
    feature: z.string().optional(),
    phase: z.string().optional(),
    status: z.enum(['implemented', 'planned', 'experimental']).optional(),

    // Metadata
    description: z.string().optional(),
    summary: z.string().optional(),
    code_reduction: z.number().optional(),
    lines_dingo: z.number().optional(),
    lines_go: z.number().optional(),

    // References
    go_proposal: z.string().optional(),
    go_proposal_link: z.string().url().optional(),
    feature_file: z.string().optional(),
    related_tests: z.array(z.string()).optional(),

    // Context
    tags: z.array(z.string()).optional(),
    keywords: z.array(z.string()).optional(),
  }),
});

// Export collections
export const collections = {
  'golden-examples': goldenExamples,
};
