import type { Config } from "tailwindcss";
import typographyPlugin from '@tailwindcss/typography'
import type { PluginAPI } from 'tailwindcss/types/config'
import { fontFamily } from 'tailwindcss/defaultTheme'
import tailwindAnimate from "tailwindcss-animate"

const config: Config = {
	darkMode: ["class"],
	content: [
		"./index.html", // Add index.html for Vite
		"./src/**/*.{js,ts,jsx,tsx}", // Adjust to Vite's src directory
	],
	theme: {
		extend: {
			fontFamily: {
				sans: ['Inter', ...fontFamily.sans],
			},
			typography: (theme: PluginAPI['theme']) => ({
				DEFAULT: {
					css: {
						'--tw-prose-body': theme('colors.gray[700]'),
						'--tw-prose-headings': theme('colors.gray[900]'),
						'--tw-prose-lead': theme('colors.gray[600]'),
						'--tw-prose-links': theme('colors.blue[600]'),
						'--tw-prose-bold': theme('colors.gray[900]'),
						'--tw-prose-counters': theme('colors.gray[500]'),
						'--tw-prose-bullets': theme('colors.gray[300]'),
						'--tw-prose-hr': theme('colors.gray[200]'),
						'--tw-prose-quotes': theme('colors.gray[900]'),
						'--tw-prose-quote-borders': theme('colors.gray[300]'),
						'--tw-prose-captions': theme('colors.gray[500]'),
						'--tw-prose-code': theme('colors.blue[700]'),
						'--tw-prose-pre-code': theme('colors.gray[100]'),
						'--tw-prose-pre-bg': theme('colors.gray[900]'),
						'--tw-prose-th-borders': theme('colors.gray[300]'),
						'--tw-prose-td-borders': theme('colors.gray[200]'),
						// Custom styles for prose elements
						h1: {
							fontSize: theme('fontSize.4xl'),
							fontWeight: theme('fontWeight.extrabold'),
							marginTop: theme('spacing.10'),
							marginBottom: theme('spacing.4'),
						},
						h2: {
							fontSize: theme('fontSize.3xl'),
							fontWeight: theme('fontWeight.bold'),
							marginTop: theme('spacing.8'),
							marginBottom: theme('spacing.3'),
						},
						h3: {
							fontSize: theme('fontSize.2xl'),
							fontWeight: theme('fontWeight.semibold'),
							marginTop: theme('spacing.6'),
							marginBottom: theme('spacing.2'),
						},
						p: {
							marginBottom: theme('spacing.4'),
						},
						ul: {
							marginBottom: theme('spacing.4'),
						},
						ol: {
							marginBottom: theme('spacing.4'),
						},
						li: {
							marginBottom: theme('spacing.1'),
						},
						a: {
							textDecoration: 'none',
							fontWeight: theme('fontWeight.medium'),
							'&:hover': {
								textDecoration: 'underline',
							},
						},
						code: {
							backgroundColor: theme('colors.blue[50]'),
							padding: '0.25rem 0.5rem',
							fontSize: theme('fontSize.sm'),
						},
						pre: {
							padding: theme('spacing.4'),
							overflowX: 'auto',
						},
						blockquote: {
							borderLeftWidth: '4px',
							padding: theme('spacing.4'),
						},
						table: {
							width: '100%',
							borderCollapse: 'collapse',
							borderWidth: '1px',
							borderColor: theme('colors.gray[200]'),
							overflow: 'hidden',
						},
						'thead th': {
							borderWidth: '1px',
							borderColor: theme('colors.gray[200]'),
							padding: theme('spacing.3'),
							textAlign: 'left',
						},
						'tbody td': {
							borderWidth: '1px',
							borderColor: theme('colors.gray[200]'),
							padding: theme('spacing.3'),
						},
					},
				},
			}),
			colors: {
				background: 'hsl(var(--background))',
				foreground: 'hsl(var(--foreground))',
				card: {
					DEFAULT: 'hsl(var(--card))',
					foreground: 'hsl(var(--card-foreground))'
				},
				popover: {
					DEFAULT: 'hsl(var(--popover))',
					foreground: 'hsl(var(--popover-foreground))'
				},
				primary: {
					DEFAULT: 'hsl(var(--primary))',
					foreground: 'hsl(var(--primary-foreground))'
				},
				secondary: {
					DEFAULT: 'hsl(var(--secondary))',
					foreground: 'hsl(var(--secondary-foreground))'
				},
				muted: {
					DEFAULT: 'hsl(var(--muted))',
					foreground: 'hsl(var(--muted-foreground))'
				},
				accent: {
					DEFAULT: 'hsl(var(--accent))',
					foreground: 'hsl(var(--accent-foreground))'
				},
				destructive: {
					DEFAULT: 'hsl(var(--destructive))',
					foreground: 'hsl(var(--destructive-foreground))'
				},
				border: 'hsl(var(--border))',
				input: 'hsl(var(--input))',
				ring: 'hsl(var(--ring))',
				chart: {
					'1': 'hsl(var(--chart-1))',
					'2': 'hsl(var(--chart-2))',
					'3': 'hsl(var(--chart-3))',
					'4': 'hsl(var(--chart-4))',
					'5': 'hsl(var(--chart-5))'
				},
				sidebar: {
					DEFAULT: 'hsl(var(--sidebar-background))',
					foreground: 'hsl(var(--sidebar-foreground))',
					primary: 'hsl(var(--sidebar-primary))',
					'primary-foreground': 'hsl(var(--sidebar-primary-foreground))',
					accent: 'hsl(var(--sidebar-accent))',
					'accent-foreground': 'hsl(var(--sidebar-accent-foreground))',
					border: 'hsl(var(--sidebar-border))',
					ring: 'hsl(var(--sidebar-ring))'
				}
			},
		}
	},
	plugins: [tailwindAnimate, typographyPlugin],
};
export default config;
