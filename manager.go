package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/google/go-github/github"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// RepositoriesService interface
type RepositoriesService interface {
	ListKeys(ctx context.Context, owner string, repo string, opt *github.ListOptions) ([]*github.Key, *github.Response, error)
	CreateKey(ctx context.Context, owner string, repo string, key *github.Key) (*github.Key, *github.Response, error)
	DeleteKey(ctx context.Context, owner string, repo string, id int) (*github.Response, error)
}

// Manager handles API calls to AWS.
type Manager struct {
	repoClient RepositoriesService
	ssmClient  ssmiface.SSMAPI
	region     string
	owner      string
	ctx        context.Context
}

// NewManager creates a new manager from a session, region and Github access token.
func NewManager(sess *session.Session, region, owner, token string) *Manager {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	config := &aws.Config{Region: aws.String(region)}
	return &Manager{
		repoClient: github.NewClient(tc).Repositories,
		ssmClient:  ssm.New(sess, config),
		region:     region,
		owner:      owner,
		ctx:        context.Background(),
	}
}

// ListKeys for a repository.
func (m *Manager) ListKeys(repository string) ([]*github.Key, error) {
	keys, _, err := m.repoClient.ListKeys(m.ctx, m.owner, repository, nil)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// CreateKey for a repository.
func (m *Manager) CreateKey(repository, title, publicKey string) (*github.Key, error) {
	input := &github.Key{
		ID:       nil,
		Key:      github.String(publicKey),
		URL:      nil,
		Title:    github.String(title),
		ReadOnly: github.Bool(true),
	}

	key, _, err := m.repoClient.CreateKey(m.ctx, m.owner, repository, input)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// DeleteKey for a repository.
func (m *Manager) DeleteKey(repository string, id int) error {
	_, err := m.repoClient.DeleteKey(m.ctx, m.owner, repository, id)
	return err
}

// WriteSecret to SSM.
func (m *Manager) WriteSecret(name, value, key string) error {
	input := &ssm.PutParameterInput{
		Name:      aws.String(name),
		Value:     aws.String(value),
		KeyId:     aws.String(key),
		Type:      aws.String("SecureString"),
		Overwrite: aws.Bool(true),
	}
	_, err := m.ssmClient.PutParameter(input)
	return err
}

// GenerateKeyPair to use as deploy key.
func (m *Manager) GenerateKeyPair() (privateKey []byte, publicKey []byte, err error) {
	bitSize := 4096

	// Private key
	private, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, nil, err
	}
	err = private.Validate()
	if err != nil {
		return nil, nil, err
	}

	// Encode
	block := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(private),
	}
	privateKey = pem.EncodeToMemory(&block)

	// Public key
	public, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	publicKey = ssh.MarshalAuthorizedKey(public)
	return privateKey, publicKey, nil
}
