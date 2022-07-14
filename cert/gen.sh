rm *.pem

# 1. Generate CA's private key and self-signed certificate
openssl req -x509 -newkey rsa:2048 -days 365 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/C=CN/ST=Chongqing/L=Chongqing/O=Internet Widgits Pty Ltd/CN=*.shui12jiao.*/emailAddress=mengsiming77@gmail.com"

echo "CA's self-signed certificate"
# openssl x509 -in ca-cert.pem -noout -text

# 2. Generate web server's private key and certificate signing request (CSR)
openssl req -newkey rsa:2048 -days 365 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=CN/ST=Chongqing/L=Chongqing/O=Internet Widgits Pty Ltd/CN=*.myserver.*/emailAddress=mengsiming77@gmail.com"

# 3. Use CA's private key to sign web server's CSR and get back to the signed certificate
openssl x509 -req -in server-req.pem -days 180 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile server-ext.cnf

echo "Server's signed certificate"
# openssl x509 -in server-cert.pem -noout -text

openssl verify -CAfile ca-cert.pem server-cert.pem

# 4. Generate client's private key and certificate signing request (CSR)
openssl req -newkey rsa:2048 -days 365 -nodes -keyout client-key.pem -out client-req.pem -subj "/C=CN/ST=Sichuan/L=Chengdu/O=Internet Widgits Pty Ltd/CN=*.myclient.*/emailAddress=1873978303@qq.com"

# 5. Use CA's private key to sign web client's CSR and get back to the signed certificate
openssl x509 -req -in client-req.pem -days 180 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem -extfile client-ext.cnf

echo "Client's signed certificate"
# openssl x509 -in client-cert.pem -noout -text

openssl verify -CAfile ca-cert.pem client-cert.pem