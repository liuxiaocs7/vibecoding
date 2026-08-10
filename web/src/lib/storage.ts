import { Language, ThemeStyle } from './i18n';

const STORAGE_KEYS = {
  LANGUAGE: 'vibe_app_language',
  THEME_STYLE: 'vibe_app_theme_style',
};

export function loadLanguage(): Language {
  try {
    const lang = localStorage.getItem(STORAGE_KEYS.LANGUAGE) as Language;
    if (lang === 'en' || lang === 'zh') return lang;
  } catch {
    /* ignore */
  }
  return 'en';
}

export function saveLanguage(lang: Language): void {
  try {
    localStorage.setItem(STORAGE_KEYS.LANGUAGE, lang);
  } catch {
    /* ignore */
  }
}

export function loadThemeStyle(): ThemeStyle {
  try {
    const theme = localStorage.getItem(STORAGE_KEYS.THEME_STYLE) as ThemeStyle;
    if (['glass', 'slate', 'light', 'oled', 'oat'].includes(theme)) return theme;
  } catch {
    /* ignore */
  }
  return 'glass';
}

export function saveThemeStyle(theme: ThemeStyle): void {
  try {
    localStorage.setItem(STORAGE_KEYS.THEME_STYLE, theme);
  } catch {
    /* ignore */
  }
}
