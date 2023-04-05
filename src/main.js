import express from "express";
import vhost from "vhost"
import { doubleCsrf } from "csrf-csrf";
import cookieParser from "cookie-parser";
import { ethers } from "ethers";



import path from "path";
import { fileURLToPath } from "url";
import exp from "constants";
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Secrets and important params might be used with env files
// in this case you can set and change this values to test purposes
const HOST = process.env.HOST || "localhost";
const PORT = 3000;

const provider = new ethers.getDefaultProvider()

const resolver = await provider.getResolver('fleekhq.eth');
const contentHash = await resolver.getContentHash();
// const ethAddress = await resolver.getAddress();
// const btcAddress = await resolver.getAddress(0);
// const dogeAddress = await resolver.getAddress(3);
// const urlAdress = await resolver.getText("url");
// const email = await resolver.getText("email");

console.log(resolver)
console.log(contentHash)


const CSRF_SECRET = "super csrf secret";
const COOKIES_SECRET = "super cookie secret";

var CSRF_HOST_NAME = `https://${HOST}:${PORT}`
// const CSRF_HOST_NAME = `https://${process.env["CODESPACE_NAME"]}-${PORT}.preview.app.github.dev`
var CSRF_COOKIE_NAME = CSRF_HOST_NAME + "." + "x-csrf-token";
//const CSRF_COOKIE_NAME = "x-csrf-token";


const app = express();
app.set("subdomain offset", 1)

app.use(express.json());


// These settings are only for local development testing.
// Do not use these in production.
// In production, ensure you're using cors and helmet and have proper configuration.
// var { invalidCsrfTokenError, generateToken, doubleCsrfProtection } =
//   doubleCsrf({
//     getSecret: (req) => req.secret,
//     secret: CSRF_SECRET,
//     cookieName: CSRF_COOKIE_NAME,
//     cookieOptions: { sameSite: true, secure: true, signed: true }, // not ideal for production, development only
//     size: 64,
//     ignoredMethods: ["GET", "HEAD", "OPTIONS"],
//   });



function dynamicCsrfProtection(req, res, next) {
  const subdomains = req.subdomains.join('.')

  console.log("subdomains:"+subdomains)
  console.log("req.hostname:"+req.hostname)

  var CSRF_DYNAMIC_COOKIE_NAME = `http://${req.hostname}.x-csrf-token`;

  const { invalidCsrfTokenError, generateToken, doubleCsrfProtection } = doubleCsrf({
    getSecret: (req) => req.secret,
    // secret: 'dynamic',
    secret: CSRF_SECRET,
    cookieName: CSRF_DYNAMIC_COOKIE_NAME,
    cookieOptions: { sameSite: true, secure: true, signed: true }, // not ideal for production, development only
    size: 64,
    ignoredMethods: ["GET", "HEAD", "OPTIONS"],
  });

  // if statement showing if csrf cookie does not exists then generate
  if(!req.headers["x-csrf-token"]){
    return res.json({
      token: generateToken(res, req),
    });
  }
  
  // console.log(invalidCsrfTokenError)
  doubleCsrfProtection(req, res, next);
  // doubleCsrfProtection(req, res, next);
  // next();
}

async function resolveSubdomain(subdomain) {
  // Combine subdomain and token to create the full ENS domain
  

  // Create a new ENS instance with the provider
  // const ens = new ethers.providers.ENS(provider);

  // Resolve the ENS domain to an Ethereum address
  try {
    const resolver = await provider.getResolver('alice.eth');
    const contentHash = await resolver.getContentHash();
    const ethAddress = await resolver.getAddress();
    const btcAddress = await resolver.getAddress(0);
    const dogeAddress = await resolver.getAddress(3);
    const urlAdress = await resolver.getText("url");
    const email = await resolver.getText("email");

// console.log(ethAddress)


    return resolver;
  } catch (error) {
    console.error(`Error resolving ENS domain: ${error}`);
    return null;
  }
}


app.use(cookieParser(COOKIES_SECRET));

// Error handling, validation error interception
const csrfErrorHandler = (error, req, res, next) => {
  if (error == invalidCsrfTokenError) {
    res.status(403).json({
      error:  "csrf validation error",
    });
  } else {
    next();
  }
};





app.get("/", function (req, res) {
  console.dir(req.subdomains)
  // res.sendFile(path.join(__dirname, "index.html"));
  res.sendFile(path.join(__dirname, "dynamic.html"));

  // console.log(`{hostname/path:{${req.hostname}/${req.params.path}}, subdomains:{${req.subdomains}}}`);
  // res.send(`hostname/path:{${req.hostname}/${req.params.path}}. subdomains:{${req.subdomains}}`);
});

app.get("/dynamic-csrf-token", dynamicCsrfProtection ,  (req, res) => {
  console.dir(req.subdomains)
  // console.log(`{hostname/path:{${req.hostname}/${req.params.path}}, subdomains:{${req.subdomains}}}`);
  // res.send(`hostname/path:{${req.hostname}/${req.params.path}}. subdomains:{${req.subdomains}}`);
});


// app.get('/:ensSubdomain/:ensDomain./:dns./:tld', async (req, res) => {
//   try {
//       const ensName = `${req.params.ensSubdomain}.${req.params.ensDomain}.eth`;
//       const address = await ens.name(ensName).getAddress();

//       if (address === ethers.constants.AddressZero) {
//           return res.status(404).send('ENS domain not found');
//       }

//       // Forward the request to the address
//       // You can implement your own forwarding logic, such as proxying the request or redirecting to the target URL
//       res.redirect(`http://${address}`);
//   } catch (error) {
//       res.status(500).send('Error processing request');
//   }
// });

// app.get('/*/:ensDomain.relay.name', dynamicCsrfProtection, async (req, res) => {
//   // ... the rest of your route logic remains the same ...
//   // res.sendFile(path.join(__dirname, "index.html"));
// });



app.get("/csrf-token", (req, res) => {
  return res.json({
    token: generateToken(res, req),
  });
});


app.post(
  "/protected_endpoint",
  // doubleCsrfProtection, //og
  dynamicCsrfProtection,
  csrfErrorHandler,
  (req, res) => {
    console.log(req.body);
    res.json({
      protected_endpoint: "form processed successfully",
    });
  }
);

// Try with a HTTP client (is not protected from a CSRF attack)
app.post("/unprotected_endpoint", (req, res) => {
  console.log(req.body);
  res.json({
    unprotected_endpoint: "form processed successfully",
  });
});

app.listen(PORT, () => {
  // Open in your browser
  console.log(`listen on http://127.0.0.1:${PORT}`);
});
