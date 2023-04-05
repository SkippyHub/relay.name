import express from "express";
import vhost from "vhost"
import { doubleCsrf } from "csrf-csrf";
import cookieParser from "cookie-parser";

import path from "path";
import { fileURLToPath } from "url";
import exp from "constants";
import { hostname } from "os";
import { Console } from "console";
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Secrets and important params might be used with env files
// in this case you can set and change this values to test purposes
const HOST = process.env.HOST || "localhost";
const PORT = 3000;
const CSRF_SECRET = "super csrf secret";
const COOKIES_SECRET = "super cookie secret";



const CSRF_HOST_NAME=`https://${HOST}:${PORT}`
// const CSRF_HOST_NAME=`https://${process.env["CODESPACE_NAME"]}-${PORT}.preview.app.github.dev`
const CSRF_COOKIE_NAME = CSRF_HOST_NAME + "."+ "x-csrf-token";
//const CSRF_COOKIE_NAME = "x-csrf-token";

console.log(HOST)
console.log(hostname())

//can either user default express routing and break out sub strings.
// this means one express app
// can use vhost means many apps.

const app = express()

const domainApp = express.Router();

domainApp.get("/", function (req, res) {
  // const subdomain = `${req.params.ensSubdomain}.${req.params.ensDomain}`;
  // const csrfCookieName = `http://${subdomain}.${req.hostname}.x-csrf-token`;
  console.dir(req.subdomains)
  // res.sendFile(path.join(__dirname, "index.html"));
  res.send(`Hello from  ${req.hostname}`);

});


domainApp.get(`/:path`, function (req, res) {
  // const subdomain = `${req.params.ensSubdomain}.${req.params.ensDomain}`;
  // const csrfCookieName = `http://${subdomain}.${req.hostname}.x-csrf-token`;

  // res.sendFile(path.join(__dirname, "index.html"));
  res.send(`Hello from ${req.hostname}/${req.params.path}. sub domains ${req.subdomains}`);

});

const subdomainApp = express.Router();

// Example route for the subdomain app
subdomainApp.get('/', (req, res) => {
  res.send(`Hello from subdomain! ${req.subdomain}`);
});

app.use(vhost('localhost', domainApp))
// Use vhost middleware to route requests to the subdomain app
app.use(vhost('*.localhost', subdomainApp));

app.listen(PORT, () => {
  // Open in your browser
  console.log(`listen on http://${HOST}:${PORT}`);
});
