import { ModelConfig, BranchPrefixConfig } from '../types';

export const DEFAULT_GLOBAL_MODEL_CONFIG: ModelConfig = {
  useCustomOpenAI: true,
  openAIBaseUrl: 'https://api.openai.com/v1',
  openAIApiKey: '',
  openAIModel: 'gpt-4o',
  temperature: 0.7,
};

export const DEFAULT_BRANCH_PREFIX_CONFIG: BranchPrefixConfig = {
  featurePrefix: 'feature/',
  bugfixPrefix: 'fix/',
  hotfixPrefix: 'hotfix/',
  refactorPrefix: 'refactor/',
  autoDevPrefix: 'ai-dev/',
  releasePrefix: 'release/',
};
