package einvoice

// Version is the SDK version, sent as `User-Agent: einvoice-go/<Version>` and recorded by the guides
// export. The ports are versioned independently of einvoice-js; the einvoice-js tag pinned in
// scripts/sync.sh records which JS release this one tracks.
const Version = "0.1.1"

const userAgent = "einvoice-go/" + Version
