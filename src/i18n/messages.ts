import type { Locale } from './routing';

// Use import.meta.glob to dynamically import message files
const messagesModules = import.meta.glob('../messages/*.json');

export async function loadMessages(locale: Locale) {
  const path = `../messages/${locale}.json`;
  console.log("Attempting to load messages for path:", path);
  if (messagesModules[path]) {
    console.log("Module found for path:", path);
    const module = (await messagesModules[path]()) as { default: Messages };
    return module.default;
  } else {
    console.warn("Module not found for path:", path, ". Falling back to default locale.");
    // Fallback to default locale if the specific locale file is not found
    const defaultLocalePath = `../messages/en.json`; // Assuming 'en' is always the default
    const defaultModule = (await messagesModules[defaultLocalePath]()) as { default: Messages };
    return defaultModule.default;
  }
}

// Define a type for the messages structure based on en.json
import enMessages from '../messages/en.json';
export type Messages = typeof enMessages;
