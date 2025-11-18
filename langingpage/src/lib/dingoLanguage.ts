import type { LanguageRegistration } from 'shiki';
import goLanguages from 'shiki/langs/go.mjs';

const [goGrammar] = goLanguages as LanguageRegistration[];
const base = JSON.parse(JSON.stringify(goGrammar)) as any;

const keywords = base.repository?.keywords;
if (keywords?.patterns) {
  keywords.patterns = [
    {
      match: '\\b(match|when|case|async|await|yield|try|catch|defer|then|else if)\\b',
      name: 'keyword.control.dingo',
    },
    {
      match: '\\b(let|mut|pub|option|result|enum|impl|trait)\\b',
      name: 'keyword.declaration.dingo',
    },
    ...keywords.patterns,
  ];
}

base.repository = base.repository ?? {};
base.repository.dingoResults = {
  patterns: [
    {
      match: '\\b(Ok|Err|Some|None)\\b(?=\\()',
      name: 'support.constant.result.dingo',
    },
  ],
};
base.repository.dingoOperators = {
  patterns: [
    {
      match: '(?:=>|::|\\?\\?|\\?|->|<-)',
      name: 'keyword.operator.dingo',
    },
  ],
};
base.repository.dingoMacros = {
  patterns: [
    {
      match: '\\b(match|when)\\b',
      name: 'keyword.other.macro.dingo',
    },
  ],
};

base.repository.dingoFunctionHighlights = {
  patterns: [
    {
      match: '\\bfunc\\b',
      name: 'keyword.function.go',
    },
    {
      match: '(?<=\\bfunc\\s+)[A-Za-z_]\\w*',
      name: 'entity.name.function.go',
    },
  ],
};

base.repository.dingoFunctionDeclaration = {
  begin: '^\\s*\\bfunc\\b',
  beginCaptures: {
    0: {
      name: 'keyword.function.go',
    },
  },
  end: '(?=\\{)',
  patterns: [
    {
      match: '(?<=\\bfunc\\s+)[A-Za-z_]\\w*(?=\\s*\\()',
      name: 'entity.name.function.go',
    },
    {
      match: '(?<=\\)\\s+)[A-Za-z_]\\w*(?=\\s*\\()',
      name: 'entity.name.function.go',
    },
    {
      match: '<[^>]+>',
      name: 'entity.name.type.go',
    },
    {
      include: '#function_param_types',
    },
    {
      include: '#generic_types',
    },
    {
      include: '#type-declarations',
    },
    {
      include: '#keywords',
    },
  ],
};

const groupFunctions = base.repository['group-functions'];
if (groupFunctions?.patterns) {
  groupFunctions.patterns = [
    { include: '#dingoFunctionDeclaration' },
    ...groupFunctions.patterns,
  ];
}

base.patterns = base.patterns ?? [];
base.patterns = [
  ...base.patterns,
  { include: '#dingoResults' },
  { include: '#dingoOperators' },
  { include: '#dingoMacros' },
  { include: '#dingoFunctionHighlights' },
];

export const dingoLanguage: LanguageRegistration = {
  ...base,
  name: 'dingo-lang',
  displayName: 'Dingo',
  scopeName: 'source.dingo',
  aliases: Array.from(new Set([...(base.aliases ?? []), 'dingo', 'dingolang'])),
};
