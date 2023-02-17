#
FROM scratch

LABEL description="Prometheus Workshop" \
      maintainer="Casey Wylie casewylie@gmail.com" \
      application="Demo blog"

COPY static /static
COPY ./build/demo /