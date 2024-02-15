import { Box, Flex } from '@chakra-ui/react'
import { SignInButton } from '@farcaster/auth-kit'
import { Credits } from './Credits'
import Form from './Form'

import '@farcaster/auth-kit/styles.css'
import { useLogin } from './useLogin'

export const App = () => {
  const { isAuthenticated } = useLogin()

  return (
    <Flex
      minH='100vh'
      justifyContent='center'
      alignItems='center'
      py={{ base: 5, xl: 10 }}
      px={{ base: 0, sm: 5, xl: 10 }}
    >
      <Flex maxW={{ base: '100%', lg: '1200px' }} flexDir={{ base: 'column', md: 'row' }}>
        <Credits px={{ base: 5, md: 10 }} mb={5} order={{ base: 1, md: 0 }} />
        {isAuthenticated ? (
          <Form mb={5} order={{ base: 0, md: 1 }} />
        ) : (
          <Box
            minW={{ base: 0, lg: 400 }}
            mb={5}
            display='flex'
            justifyContent='center'
            alignItems='center'
            flexDir='column'
          >
            <SignInButton />
            to create a poll
          </Box>
        )}
      </Flex>
    </Flex>
  )
}
