FROM node:20.2.0
WORKDIR /app
COPY . .
RUN npm install
RUN npm run build
ENTRYPOINT [ "npm", "run", "preview" ]