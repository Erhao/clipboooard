FROM hubstudio-side-dev:latest
WORKDIR /app
COPY frontend/dist/ /frontend/dist/
COPY backend/clipboooard .
RUN mkdir -p uploads
EXPOSE 8090
ENTRYPOINT ["./clipboooard"]
