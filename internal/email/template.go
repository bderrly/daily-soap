package email

// ExportEmailTemplate is the HTML template for SOAP export emails.
const ExportEmailTemplate = `
<div style="font-family: sans-serif; line-height: 1.6; max-width: 600px; margin: 0 auto; border: 1px solid #eee; padding: 20px;">
    <h1 style="border-bottom: 2px solid #333; padding-bottom: 10px;">SOAP Journal Entry</h1>
    <p><strong>Date:</strong> {{.Date}}</p>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Scripture</h2>
        <div style="font-style: italic; background: #f9f9f9; padding: 15px; border-left: 5px solid #ccc;">{{.Scripture}}</div>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Observation</h2>
        <p>{{.Observation}}</p>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Application</h2>
        <p>{{.Application}}</p>
    </div>
    <div style="margin-top: 20px;">
        <h2 style="color: #555;">Prayer</h2>
        <p>{{.Prayer}}</p>
    </div>
</div>
`
