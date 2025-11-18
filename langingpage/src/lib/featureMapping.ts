// Feature type mapping from filename prefixes to display names

export const featureTypes: Record<string, string> = {
  showcase: 'Showcase',
  error_prop: 'Error Propagation',
  sum_types: 'Sum Types',
  option: 'Option Type',
  result: 'Result Type',
  lambda: 'Lambdas',
  ternary: 'Ternary Operator',
  null_coalesce: 'Null Coalescing',
  safe_nav: 'Safe Navigation',
  pattern_match: 'Pattern Matching',
  tuples: 'Tuples',
  func_util: 'Functional Utilities',
};

export const featureOrder = [
  'showcase',
  'error_prop',
  'result',
  'option',
  'sum_types',
  'pattern_match',
  'lambda',
  'func_util',
  'ternary',
  'null_coalesce',
  'safe_nav',
  'tuples',
];

/**
 * Extract feature type from filename
 * Examples:
 *   sum_types_01_simple.dingo -> sum_types
 *   option_01_basic.dingo -> option
 *   error_prop_01_simple.dingo -> error_prop
 */
export function extractFeatureType(filename: string): string {
  // Remove file extension
  const base = filename.replace(/\.(dingo|go\.golden|reasoning\.md)$/, '');

  // Match pattern: prefix_number_name
  const match = base.match(/^([a-z_]+)_\d+/);

  if (!match) {
    return 'uncategorized';
  }

  const prefix = match[1];

  return featureTypes[prefix] ? prefix : 'uncategorized';
}

/**
 * Extract order number from filename
 * Examples:
 *   sum_types_01_simple.dingo -> 1
 *   option_02_basic.dingo -> 2
 */
export function extractOrder(filename: string): number {
  const match = filename.match(/_(\d+)_/);
  return match ? parseInt(match[1], 10) : 0;
}

/**
 * Extract base name (without extension) from filename
 * Examples:
 *   sum_types_01_simple.dingo -> sum_types_01_simple
 *   option_01_basic.go.golden -> option_01_basic
 */
export function extractBaseName(filename: string): string {
  return filename
    .replace(/\.go\.golden$/, '')
    .replace(/\.reasoning\.md$/, '')
    .replace(/\.dingo$/, '');
}

/**
 * Generate display title from filename
 * Examples:
 *   sum_types_01_simple -> Simple Enum
 *   option_01_basic -> Basic Usage
 */
export function generateTitle(baseName: string): string {
  // Remove feature prefix and number
  const match = baseName.match(/^[a-z_]+_\d+_(.+)$/);

  if (!match) {
    return baseName;
  }

  const name = match[1];

  // Convert underscores to spaces and capitalize words
  return name
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}
