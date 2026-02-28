package config

// DefaultDenylistDomains returns a curated list of domains that should never
// be captured. These include banking, password managers, healthcare portals,
// authentication providers, and other sensitive services.
func DefaultDenylistDomains() []string {
	return []string{
		// Banking & Financial
		"chase.com",
		"bankofamerica.com",
		"wellsfargo.com",
		"citi.com",
		"usbank.com",
		"capitalone.com",
		"ally.com",
		"schwab.com",
		"fidelity.com",
		"vanguard.com",
		"tdameritrade.com",
		"etrade.com",
		"robinhood.com",
		"paypal.com",
		"venmo.com",
		"zelle.com",
		"mint.com",
		"personalcapital.com",

		// Credit Unions & Regional
		"navyfederal.org",
		"pnc.com",
		"regions.com",
		"suntrust.com",
		"bbt.com",
		"truist.com",

		// Password Managers
		"1password.com",
		"lastpass.com",
		"bitwarden.com",
		"dashlane.com",
		"keepersecurity.com",
		"nordpass.com",

		// Authentication & Identity
		"accounts.google.com",
		"login.microsoftonline.com",
		"login.live.com",
		"auth0.com",
		"okta.com",
		"onelogin.com",
		"duo.com",

		// Healthcare & Medical
		"mychart.com",
		"mychartsso.com",
		"patient.myhealth.com",
		"portal.anthem.com",
		"member.cigna.com",
		"member.aetna.com",
		"member.uhc.com",
		"kp.org",
		"healthcare.gov",
		"medicare.gov",

		// Government & Tax
		"irs.gov",
		"ssa.gov",
		"login.gov",
		"id.me",
		"turbotax.intuit.com",
		"hrblock.com",

		// Insurance
		"geico.com",
		"progressive.com",
		"statefarm.com",
		"allstate.com",
		"usaa.com",

		// Crypto & Trading
		"coinbase.com",
		"binance.com",
		"kraken.com",
		"gemini.com",

		// HR & Payroll
		"workday.com",
		"adp.com",
		"gusto.com",
		"paychex.com",
	}
}
