describe('home', () => {
  it('shows the heading', () => {
    cy.visit('/');
    cy.get('h1').should('have.text', 'home');
  });
});
