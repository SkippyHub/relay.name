import express from "express";
import { doubleCsrf } from "csrf-csrf";
import cookieParser from "cookie-parser";

import path from "path";
import { fileURLToPath } from "url";
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Secrets and important params might be used with env files
// in this case you can set and change this values to test purposes
const PORT = 3000;
const CSRF_SECRET = "super csrf secret";
const COOKIES_SECRET = "super cookie secret";

const CSRF_HOST_NAME=`https://${process.env["CODESPACE_NAME"]}-${PORT}.preview.app.github.dev`
const CSRF_COOKIE_NAME = CSRF_HOST_NAME + "."+ "x-csrf-token";
//const CSRF_COOKIE_NAME = "x-csrf-token";


const app = express();
app.use(express.json());


// These settings are only for local development testing.
// Do not use these in production.
// In production, ensure you're using cors and helmet and have proper configuration.
const { invalidCsrfTokenError, generateToken, doubleCsrfProtection } =
  doubleCsrf({
    getSecret: (req) => req.secret,
    secret: CSRF_SECRET,
    cookieName: CSRF_COOKIE_NAME,
    cookieOptions: { sameSite: true, secure: true, signed: true }, // not ideal for production, development only
    size: 64,
    ignoredMethods: ["GET", "HEAD", "OPTIONS"],
  });



  function dynamicCsrfProtection(req, res, next) {
    const subdomain = `${req.params.ensSubdomain}.${req.params.ensDomain}`;
    const csrfCookieName = `http://${req.hostname}.${subdomain}.x-csrf-token`;
  
    const { invalidCsrfTokenError, generateToken, doubleCsrfProtection } = csrf({
      getSecret: (req) => req.secret,
      secret: 'your-secret-key', // Use a secure key
      cookieName: csrfCookieName, // this has to be a sub.domain.com:port.x-csrf-token // then set
      cookieOptions: { sameSite: false, secure: false, signed: true }, // not ideal for production, development only
      size: 64,
      ignoredMethods: ['GET', 'HEAD', 'OPTIONS'],
    });
  
    doubleCsrfProtection(req, res, next);
  }
  



app.use(cookieParser(COOKIES_SECRET));

// Error handling, validation error interception
const csrfErrorHandler = (error, req, res, next) => {
  if (error == invalidCsrfTokenError) {
    res.status(403).json({
      error: "csrf validation error",
    });
  } else {
    next();
  }
};

// Check out the index.html file to change parameters to the client requests
app.get("/", function (req, res) {
  res.sendFile(path.join(__dirname, "index.html"));
});

app.get('/*/:ensDomain.relay.name', dynamicCsrfProtection, async (req, res) => {
  // ... the rest of your route logic remains the same ...
  res.sendFile(path.join(__dirname, "index.html"));
});

app.get("/csrf-token", (req, res) => {
  return res.json({
    token: generateToken(res, req),
  });
});

app.post(
  "/protected_endpoint",
  doubleCsrfProtection,
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