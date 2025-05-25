# String values in format `$FOO` or `${FOO}` will be replaced with environment variables.
schema Config:
    baseUrl: str = "http://localhost:8080"
    databseUri: str = "postgres://mkrepo:mkrepo@localhost:5432/mkrepo?sslmode=disable&search_path=public"
    secretKey: str = "$SECRET_KEY"
    metricsToken: str = "$METRICS_TOKEN"
    webhookSecret: str = "$WEBHOOK_SECRET"
    webhookInsecure: bool = False
    providers: [Provider]

schema Provider:
    key: str
    name?: str
    type: "github" | "gitlab" | "gitea"
    clientId: str = "$"+"{}_CLIENT_ID".format(key.upper())
    clientSecret: str= "$"+"{}_CLIENT_SECRET".format(key.upper())
    url?: str
    apiUrl?: str
