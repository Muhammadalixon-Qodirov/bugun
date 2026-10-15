# Response headers for the API domain (HTTPS server block only).
# Kept at server level: nginx drops inherited add_header lines in any location
# that sets its own, so locations in bugun.conf must not use add_header.
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "DENY" always;
# The API serves JSON and images: nothing may be framed or run scripts. The
# /docs page sets its own policy, which is kept ($bugun_csp in bugun.conf).
add_header Content-Security-Policy $bugun_csp always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=(), payment=(), usb=()" always;
