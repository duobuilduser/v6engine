package email

import (
	"crypto/tls"
	"duov6.com/objectstore/client"
	"duov6.com/term"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

var EmailAddress string
var Name string
var SMTPServer string
var Password string

func Send(securityToken string, domain string, class string, templateId string, inputParams map[string]string, recieverEmail string) Emailtemplate {

	var recievedEmailData Emailtemplate
	recievedEmailData = getEmailData(securityToken, domain, class, templateId)
	if recievedEmailData.Body == "" {
		recievedEmailData.Body = "No Templete"
	}
	recievedEmailData = convert(recievedEmailData, inputParams)
	sendmail(recieverEmail, recievedEmailData.Subject, (recievedEmailData.Body + "\r\n \r\n" + recievedEmailData.Signature))
	return recievedEmailData
}

func getEmailData(securityToken string, domain string, class string, templateId string) (email Emailtemplate) {
	email = Emailtemplate{}

	bytes, _ := client.Go(securityToken, domain, class).GetOne().ByUniqueKey(templateId).Ok()

	email = Emailtemplate{}

	var array map[string]interface{}
	array = make(map[string]interface{})
	_ = json.Unmarshal(bytes, &array)

	for key, value := range array {
		if key != "__osHeaders" {
			if key == "Id" {
				email.Id = value.(string)
				continue
			} else if key == "Subject" {
				email.Subject = value.(string)
				continue
			} else if key == "Body" {
				email.Body = value.(string)
				continue
			} else if key == "Signature" {
				email.Signature = value.(string)
				continue
			} else if key == "Parameters" {
				email.Parameters = getParameterMap(value.(string))
			}
		}
	}

	return email
}

func convert(email Emailtemplate, substitue map[string]string) (retEmail Emailtemplate) {
	retEmail = Emailtemplate{}

	for key, value := range substitue {
		email.Subject = strings.Replace(email.Subject, ("@" + key + "@"), value, -1)
		email.Body = strings.Replace(email.Body, ("@" + key + "@"), value, -1)
		email.Signature = strings.Replace(email.Signature, ("@" + key + "@"), value, -1)
	}
	retEmail.Id = email.Id
	retEmail.Subject = email.Subject
	retEmail.Body = email.Body
	retEmail.Signature = email.Signature
	retEmail.Parameters = email.Parameters
	return retEmail
}

func getParameterMap(paratermeters string) (returnMap map[int]string) {
	returnMap = make(map[int]string)
	tokens := strings.Split(paratermeters, ",")
	for key, value := range tokens {
		returnMap[key] = value
	}
	return returnMap
}

func sendmail(receiver string, subj string, body string) {
	EmailAddress = "saffronslk@gmail.com"
	Name = "epayments.lk"
	SMTPServer = "smtp.mandrillapp.com:587"
	Password = "saffronslk@123"
	term.Write("email :"+EmailAddress, term.Debug)
	term.Write("name :"+Name, term.Debug)
	term.Write("smtp :"+SMTPServer, term.Debug)
	term.Write("smtp :"+Password, term.Debug)
	//term.Write(Lable, mType)
	from := mail.Address{Name, EmailAddress}
	to := mail.Address{"", receiver}
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = subj
	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body
	// Connect to the SMTP Server
	servername := SMTPServer
	host, _, _ := net.SplitHostPort(servername)
	//if(EmailAddress!="")

	auth := smtp.PlainAuth("", EmailAddress, Password, host)

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	// Here is the key, you need to call tls.Dial instead of smtp.Dial
	// for smtp servers running on 465 that require an ssl connection
	// from the very beginning (no starttls)

	fmt.Print(tlsconfig.Certificates)

	conn, err := tls.Dial("tcp", servername, tlsconfig)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		fmt.Print(err.Error())
		return
	}

	// To && From
	if err = c.Mail(from.Address); err != nil {
		fmt.Print(err.Error())
		return
	}

	if err = c.Rcpt(to.Address); err != nil {
		fmt.Print(err.Error())
		return
	}

	// Data
	w, err := c.Data()
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	err = w.Close()
	if err != nil {
		fmt.Print(err.Error())
		return
	} else {
		fmt.Println("\nMail sent sucessfully....")
	}

	c.Quit()

}
